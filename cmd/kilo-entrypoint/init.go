package main

import (
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/mbabic84/kilo-docker/pkg/constants"
)

// runInit performs container initialization when invoked with no subcommand.
//
// If running as root (UID 0), it:
//   - Creates/updates the kilo user with PUID/PGID from environment
//   - Installs enabled services from KD_SERVICES env var
//   - Fixes SSH agent socket ownership
//   - Pre-populates known_hosts for GitHub, GitLab, Bitbucket
//   - Creates config directories under ~/.config/kilo/
//   - Drops privileges via syscall.Setuid/Setgid
//   - Launches ainstruct-sync in background if KD_AINSTRUCT_ENABLED=1
//
// After privilege drop, it applies MCP server config, copies Zellij config,
// and execs into the requested command (or /bin/sh if no args).
func runInit() error {
	puidStr := os.Getenv("PUID")
	if puidStr == "" {
		puidStr = "1000"
	}
	pgidStr := os.Getenv("PGID")
	if pgidStr == "" {
		pgidStr = "1000"
	}
	puid, _ := strconv.Atoi(puidStr)
	pgid, _ := strconv.Atoi(pgidStr)

	if os.Getuid() == 0 {
		if puid != 1000 || pgid != 1000 {
			exec.Command("deluser", "kilo-t8x3m7kp").Run()
			exec.Command("addgroup", "-g", pgidStr, "kilo-t8x3m7kp").Run()
			cmd := exec.Command("adduser", "-u", puidStr, "-G", "kilo-t8x3m7kp", "-D", "-s", "/bin/sh", "kilo-t8x3m7kp")
			cmd.Stdout = os.Stderr
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("adduser failed: %w", err)
			}
		}

		if err := installServices(); err != nil {
			fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: service installation error: %v\n", err)
		}

		// Setup groups for services that require socket access
		if err := setupServiceGroups(); err != nil {
			fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: group setup error: %v\n", err)
		}

		if sshAuthSock := os.Getenv("SSH_AUTH_SOCK"); sshAuthSock != "" {
			if info, err := os.Stat(sshAuthSock); err == nil && info.Mode()&os.ModeSocket != 0 {
				if err := os.Chown(sshAuthSock, puid, pgid); err != nil {
					fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to chown SSH socket: %v\n", err)
				}
				if err := os.Chmod(sshAuthSock, 0600); err != nil {
					fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to chmod SSH socket: %v\n", err)
				}
				// Use net.DialTimeout instead of os.OpenFile to test socket connectivity.
				// os.OpenFile with O_RDWR can falsely fail on Unix sockets even when
				// they are fully functional for SSH agent communication.
				if conn, err := net.DialTimeout("unix", sshAuthSock, 0); err != nil {
					fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: SSH socket not accessible after fix: %v\n", err)
				} else {
					conn.Close()
					fmt.Fprintf(os.Stderr, "[kilo-docker] SSH agent socket ready: %s\n", sshAuthSock)
				}
			} else {
				fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: SSH_AUTH_SOCK=%s is not a valid socket\n", sshAuthSock)
			}
		}

		if err := setupKnownHosts(); err != nil {
			fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to setup known_hosts: %v\n", err)
		}

		configDirs := []string{
			"/home/kilo-t8x3m7kp/.config/kilo/commands",
			"/home/kilo-t8x3m7kp/.config/kilo/agents",
			"/home/kilo-t8x3m7kp/.config/kilo/plugins",
			"/home/kilo-t8x3m7kp/.config/kilo/skills",
			"/home/kilo-t8x3m7kp/.config/kilo/tools",
			"/home/kilo-t8x3m7kp/.config/kilo/rules",
		}
		for _, dir := range configDirs {
			os.MkdirAll(dir, 0755)
		}

		chownRecursive("/home/kilo-t8x3m7kp", puid, pgid)

		syscall.Setgid(pgid)
		syscall.Setuid(puid)

		// Set correct user identity environment variables after privilege drop.
		// These must be set before any os.Environ() call so they appear in the
		// environment passed to child processes (sudo, exec).
		kiloHome := "/home/kilo-t8x3m7kp"
		os.Setenv("HOME", kiloHome)
		os.Setenv("USER", "kilo-t8x3m7kp")
		os.Setenv("LOGNAME", "kilo-t8x3m7kp")
		if _, err := os.Stat("/bin/bash"); err == nil {
			os.Setenv("SHELL", "/bin/bash")
		} else {
			os.Setenv("SHELL", "/bin/sh")
		}
	}

	// Start ainstruct sync in background after privilege drop.
	// Must run as kilo user (not root) and must not block init.
	if os.Getenv("KD_AINSTRUCT_ENABLED") == "1" {
		cmd := exec.Command("kilo-entrypoint", "sync")
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		cmd.Start()
		fmt.Fprintf(os.Stderr, "[kilo-docker] Ainstruct sync started\n")
	}

	if err := runConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: config error: %v\n", err)
	}

	home := constants.GetHomeDir()

	if err := copyServiceConfigs(home); err != nil {
		fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to copy service configs: %v\n", err)
	}

	// Use known absolute path instead of os.Executable() which can fail
	// in containers (especially with --init / tini as PID 1) when /proc/self/exe
	// doesn't resolve correctly.
	binaryPath := "/usr/local/bin/kilo-entrypoint"

	// Validate binary exists before exec
	if _, err := os.Stat(binaryPath); err != nil {
		return fmt.Errorf("entrypoint binary not found at %s: %w", binaryPath, err)
	}

	if len(os.Args) <= 1 {
		// Keep container alive — zellij is started via docker exec from the host.
		return syscall.Exec("/bin/sleep", []string{"sleep", "infinity"}, os.Environ())
	}

	return syscall.Exec(binaryPath, os.Args[1:], os.Environ())
}

// servicesMarkerPath is the file used to track which services have been
// installed. It is stored in /tmp (container filesystem, not the persistent
// volume) so that it survives container restarts but is lost on container
// recreation — at which point the ephemeral /usr/local/bin/ binaries are
// also gone and services must be reinstalled.
// Exported as a variable so tests can override it with a temporary path.
var servicesMarkerPath = "/tmp/.kilo-services-installed"

// runInstallCmd executes a shell command for service installation.
// Exported as a variable so tests can stub it out.
var runInstallCmd = func(cmd string) error {
	c := exec.Command("sh", "-c", cmd)
	c.Stdout = os.Stderr
	c.Stderr = os.Stderr
	return c.Run()
}

// installServices reads KD_SERVICES env var and runs install commands for each enabled service.
// On subsequent starts with the same set of services, installation is skipped entirely.
func installServices() error {
	servicesEnv := os.Getenv("KD_SERVICES")
	if servicesEnv == "" {
		return nil
	}

	// Check marker file — if the same services were already installed, skip.
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

	// Write marker file so next start skips installation.
	if err := os.WriteFile(servicesMarkerPath, []byte(servicesEnv+"\n"), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to write services marker: %v\n", err)
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
// group membership for services that require socket access. This must run as
// root before privilege drop so that addgroup can modify /etc/group.
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
			exec.Command("addgroup", "kilo-t8x3m7kp", svc.Name).Run()
			continue
		}
		cmd2 := exec.Command("getent", "group", gid)
		out, err := cmd2.Output()
		if err != nil {
			continue
		}
		parts := strings.SplitN(string(out), ":", 2)
		if len(parts) > 0 && parts[0] != "" {
			exec.Command("addgroup", "kilo-t8x3m7kp", parts[0]).Run()
		}
	}
	return nil
}

// setupKnownHosts runs ssh-keyscan to pre-populate ~/.ssh/known_hosts
// for GitHub, GitLab, and Bitbucket, avoiding interactive host key prompts.
func setupKnownHosts() error {
	home := "/home/kilo-t8x3m7kp"
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
// It skips individual files and directories already owned by the target
// user, but continues walking into directories so that any root-owned
// subdirectories created later (e.g. by Docker volume mounts or external
// processes) are still fixed.
func chownRecursive(path string, uid, gid int) {
	filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		// Never follow symlinks
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		info, serr := d.Info()
		if serr != nil {
			return nil
		}
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			if int(stat.Uid) == uid && int(stat.Gid) == gid {
				// Already correct ownership — skip this entry, but continue
				// walking into subdirectories in case they contain root-owned files.
				return nil
			}
			os.Chown(p, uid, gid)
		}
		return nil
	})
}
