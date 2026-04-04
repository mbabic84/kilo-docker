package main

import (
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// runInit performs container initialization when invoked with no subcommand.
// It runs as root and handles infrastructure setup:
//   - Installs enabled services from KD_SERVICES env var
//   - Sets up service groups for socket access
//   - Validates SSH agent socket
//
// User creation, home directory, and privilege drop are handled by
// runUserInit() when docker exec calls kilo-entrypoint zellij-attach.
func runInit() error {
	fmt.Fprintf(os.Stderr, "[kilo-docker] Container initializing\n")
	if os.Getuid() == 0 {
		fmt.Fprintf(os.Stderr, "[kilo-docker] Running as root (UID=0)\n")
		if err := installServices(); err != nil {
			fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: service installation error: %v\n", err)
		}

		if err := setupServiceGroups(); err != nil {
			fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: group setup error: %v\n", err)
		}

		if sshAuthSock := os.Getenv("SSH_AUTH_SOCK"); sshAuthSock != "" {
			if info, err := os.Stat(sshAuthSock); err == nil && info.Mode()&os.ModeSocket != 0 {
				if conn, err := net.DialTimeout("unix", sshAuthSock, 0); err != nil {
					fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: SSH socket not accessible: %v\n", err)
				} else {
					conn.Close()
					fmt.Fprintf(os.Stderr, "[kilo-docker] SSH agent socket ready: %s\n", sshAuthSock)
				}
			} else {
				fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: SSH_AUTH_SOCK=%s is not a valid socket\n", sshAuthSock)
			}
		}
	}

	// Use known absolute path instead of os.Executable() which can fail
	// in containers (especially with --init / tini as PID 1) when /proc/self/exe
	// doesn't resolve correctly.
	binaryPath := "/usr/local/bin/kilo-entrypoint"

	if _, err := os.Stat(binaryPath); err != nil {
		return fmt.Errorf("entrypoint binary not found at %s: %w", binaryPath, err)
	}

	if len(os.Args) <= 1 {
		// Keep container alive — zellij is started via docker exec from the host.
		fmt.Fprintf(os.Stderr, "[kilo-docker] Init complete, waiting for exec\n")
		return syscall.Exec("/bin/sleep", []string{"sleep", "infinity"}, os.Environ())
	}

	return syscall.Exec(binaryPath, os.Args[1:], os.Environ())
}

// servicesMarkerPath is the file used to track which services have been
// installed. It is stored in /tmp (container filesystem, not the persistent
// volume) so that it survives container restarts but is lost on container
// recreation — at which point the ephemeral /usr/local/bin/ binaries are
// also gone and services must be reinstalled.
var servicesMarkerPath = "/tmp/.kilo-services-installed"

// runInstallCmd executes a shell command for service installation.
var runInstallCmd = func(cmd string) error {
	c := exec.Command("sh", "-c", cmd)
	c.Stdout = os.Stderr
	c.Stderr = os.Stderr
	return c.Run()
}

// installServices reads KD_SERVICES env var and runs install commands for each enabled service.
func installServices() error {
	servicesEnv := os.Getenv("KD_SERVICES")
	if servicesEnv == "" {
		return nil
	}

	if existing, err := os.ReadFile(servicesMarkerPath); err == nil && strings.TrimSpace(string(existing)) == servicesEnv {
		fmt.Fprintf(os.Stderr, "[kilo-docker] KD_SERVICES=%s (already installed)\n", servicesEnv)
		return nil
	}

	fmt.Fprintf(os.Stderr, "[kilo-docker] KD_SERVICES=%s\n", servicesEnv)
	for _, svcName := range strings.Split(servicesEnv, ",") {
		svc := getService(svcName)
		if svc == nil {
			fmt.Fprintf(os.Stderr, "[kilo-docker] Service %q not found in builtInServices\n", svcName)
			continue
		}
		for _, installCmd := range svc.Install {
			if installCmd == "" {
				continue
			}
			fmt.Fprintf(os.Stderr, "[kilo-docker] Installing %s: %s\n", svc.Name, installCmd)
			if err := runInstallCmd(installCmd); err != nil {
				fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to install %s: %v\n", svc.Name, err)
			}
		}
	}

	if err := os.WriteFile(servicesMarkerPath, []byte(servicesEnv+"\n"), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to write services marker: %v\n", err)
	}

	return nil
}

// installUserServices runs UserInstall commands for services that require
// the user's home directory. Called from runUserInit() after user creation
// but before privilege drop, with HOME set to the actual user home.
func installUserServices(homeDir string) error {
	servicesEnv := os.Getenv("KD_SERVICES")
	if servicesEnv == "" {
		return nil
	}

	for _, svcName := range strings.Split(servicesEnv, ",") {
		svc := getService(svcName)
		if svc == nil || len(svc.UserInstall) == 0 {
			continue
		}
		for _, installCmd := range svc.UserInstall {
			if installCmd == "" {
				continue
			}
			fmt.Fprintf(os.Stderr, "[kilo-docker] User-installing %s: %s\n", svc.Name, installCmd)
			c := exec.Command("sh", "-c", installCmd)
			c.Env = append(os.Environ(), "HOME="+homeDir)
			c.Stdout = os.Stderr
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to user-install %s: %v\n", svc.Name, err)
			}
		}
	}
	return nil
}

// copyServiceConfigs copies configured files from the container filesystem
// to the user's home directory for each enabled service.
func copyServiceConfigs(home string) error {
	servicesEnv := os.Getenv("KD_SERVICES")
	if servicesEnv == "" {
		return nil
	}

	for _, svcName := range strings.Split(servicesEnv, ",") {
		svc := getService(svcName)
		if svc == nil {
			continue
		}
		for _, cfg := range svc.CopyConfigs {
			dst := expandHome(cfg.Dst, home)
			if dst == "" {
				continue
			}
			if _, err := os.Stat(dst); err == nil {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
				continue
			}
			src, err := os.Open(cfg.Src)
			if err != nil {
				continue
			}
			defer src.Close()
			f, err := os.Create(dst)
			if err != nil {
				continue
			}
			defer f.Close()
			io.Copy(f, src)
		}
	}
	return nil
}

// expandHome replaces "~" in path with the given home directory.
func expandHome(path, home string) string {
	if len(path) >= 2 && path[0] == '~' && path[1] == '/' {
		return filepath.Join(home, path[2:])
	}
	return path
}

// setupServiceGroups reads KD_SERVICES and DOCKER_GID env vars, then sets up
// group membership for services that require socket access.
func setupServiceGroups() error {
	servicesEnv := os.Getenv("KD_SERVICES")
	if servicesEnv == "" {
		return nil
	}

	for _, svcName := range strings.Split(servicesEnv, ",") {
		svc := getService(svcName)
		if svc == nil || svc.RequiresSocket == "" {
			continue
		}
		gid := os.Getenv(svc.GIDEnvVar)
		if gid == "" {
			continue
		}
		cmd := exec.Command("addgroup", "-g", gid, svc.Name)
		if err := cmd.Run(); err == nil {
			continue
		}
		cmd2 := exec.Command("getent", "group", gid)
		out, err := cmd2.Output()
		if err != nil {
			continue
		}
		parts := strings.SplitN(string(out), ":", 2)
		if len(parts) > 0 && parts[0] != "" {
			_ = parts[0] // group exists, service groups will be joined in userinit
		}
	}
	return nil
}

// setupKnownHosts runs ssh-keyscan to pre-populate ~/.ssh/known_hosts
// for GitHub, GitLab, and Bitbucket, avoiding interactive host key prompts.
func setupKnownHosts(home string) error {
	sshDir := filepath.Join(home, ".ssh")
	os.MkdirAll(sshDir, 0700)

	knownHosts := filepath.Join(sshDir, "known_hosts")
	f, err := os.OpenFile(knownHosts, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	cmd := exec.Command("ssh-keyscan", "-H", "github.com", "gitlab.com", "bitbucket.com")
	cmd.Stdout = f
	cmd.Stderr = io.Discard
	cmd.Run()
	return nil
}

// chownRecursive changes ownership of path and its contents to uid:gid.
func chownRecursive(path string, uid, gid int) {
	filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		info, serr := d.Info()
		if serr != nil {
			return nil
		}
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			if int(stat.Uid) == uid && int(stat.Gid) == gid {
				return nil
			}
			os.Chown(p, uid, gid)
		}
		return nil
	})
}

