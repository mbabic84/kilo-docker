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

	"github.com/mbabic84/kilo-docker/pkg/utils"
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
	utils.Log("Container initializing\n")
	if os.Getuid() == 0 {
		utils.Log("Running as root (UID=0)\n")
		if err := installServices(); err != nil {
			utils.LogWarn("service installation error: %v\n", err)
		}

		if err := setupServiceGroups(); err != nil {
			utils.LogWarn("group setup error: %v\n", err)
		}

		if sshAuthSock := os.Getenv("SSH_AUTH_SOCK"); sshAuthSock != "" {
			if info, err := os.Stat(sshAuthSock); err == nil && info.Mode()&os.ModeSocket != 0 {
				if conn, err := net.DialTimeout("unix", sshAuthSock, 0); err != nil {
					utils.LogWarn("SSH socket not accessible: %v\n", err)
				} else {
					conn.Close()
					utils.Log("SSH agent socket ready: %s\n", sshAuthSock)
				}
			} else {
				utils.LogWarn("SSH_AUTH_SOCK=%s is not a valid socket\n", sshAuthSock)
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
		utils.Log("Init complete, waiting for exec\n")
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
		utils.Log("KD_SERVICES=%s (already installed)\n", servicesEnv)
		return nil
	}

	utils.Log("Installing system-scoped services: %s\n", servicesEnv)
	for _, svcName := range strings.Split(servicesEnv, ",") {
		svc := getService(svcName)
		if svc == nil {
			utils.LogError("Service %q not found in builtInServices\n", svcName)
			continue
		}
		for _, installCmd := range svc.Install {
			if installCmd == "" {
				continue
			}
			if err := runInstallCmd(installCmd); err != nil {
				utils.LogWarn("Installing %s: error: %v\n", svc.Name, err)
			} else {
				utils.Log("Installing %s: ok\n", svc.Name)
			}
		}
	}

	if err := os.WriteFile(servicesMarkerPath, []byte(servicesEnv+"\n"), 0644); err != nil {
		utils.LogWarn("failed to write services marker: %v\n", err)
	}

	return nil
}

// runVersionCheck executes a command and returns its trimmed output.
func runVersionCheck(cmd string, homeDir string) string {
	if cmd == "" {
		return ""
	}
	c := exec.Command("sh", "-c", cmd)
	c.Env = append(os.Environ(), "HOME="+homeDir)
	c.Stderr = nil
	out, err := c.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// compareVersions compares two version strings. Returns:
//   -1 if v1 < v2 (v1 is older)
//    0 if v1 == v2
//    1 if v1 > v2 (v1 is newer)
func compareVersions(v1, v2 string) int {
	if v1 == v2 {
		return 0
	}
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")
	for i := 0; i < max(len(parts1), len(parts2)); i++ {
		p1 := 0
		p2 := 0
		if i < len(parts1) {
			p1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			p2, _ = strconv.Atoi(parts2[i])
		}
		if p1 < p2 {
			return -1
		}
		if p1 > p2 {
			return 1
		}
	}
	return 0
}

// promptYesNo prompts the user with a question and returns true if they answered yes.
func promptYesNo(question string) bool {
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", question)
	var answer string
	fmt.Scanln(&answer)
	return strings.ToLower(strings.TrimSpace(answer)) == "y"
}

// installUserServices runs UserInstall commands for services that require
// the user's home directory. Called from runUserInit() after user creation
// but before privilege drop, with HOME set to the actual user home.
func installUserServices(homeDir string) error {
	servicesEnv := os.Getenv("KD_SERVICES")
	if servicesEnv == "" {
		return nil
	}

	var userServices []string
	for _, svcName := range strings.Split(servicesEnv, ",") {
		svc := getService(svcName)
		if svc == nil || len(svc.UserInstall) == 0 {
			continue
		}
		userServices = append(userServices, svc.Name)
	}

	if len(userServices) > 0 {
		utils.Log("Installing user-scoped services: %s\n", strings.Join(userServices, ", "))
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

			currentVer := runVersionCheck(svc.VersionCheck, homeDir)
			latestVer := runVersionCheck(svc.LatestVersion, homeDir)

			if currentVer != "" && latestVer != "" {
				if compareVersions(currentVer, latestVer) < 0 {
					utils.Log("Updating %s: %s -> %s\n", svc.Name, currentVer, latestVer)
					if !promptYesNo(fmt.Sprintf("Update %s?", svc.Name)) {
						utils.Log("Skipping %s update\n", svc.Name)
						continue
					}
				} else {
					utils.Log("Skipping %s: already at latest version (%s)\n", svc.Name, currentVer)
					continue
				}
			} else {
				if currentVer == "" && latestVer != "" {
					utils.Log("Installing %s (current: none, latest: %s)\n", svc.Name, latestVer)
				} else {
					utils.Log("Installing %s\n", svc.Name)
				}
			}

			c := exec.Command("sh", "-c", installCmd)
			c.Env = append(os.Environ(), "HOME="+homeDir)
			c.Stdout = os.Stderr
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				utils.LogWarn("Installing %s: error: %v\n", svc.Name, err)
			} else {
				utils.Log("Installing %s: ok\n", svc.Name)
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

