package main

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// runInit performs container initialization when invoked with no subcommand.
//
// If running as root (UID 0), it:
//   - Creates/updates the kilo user with PUID/PGID from environment
//   - Downloads Docker client, Compose, and Zellij if enabled
//   - Sets up Docker group membership
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

		if os.Getenv("DOCKER_ENABLED") == "1" {
			if err := downloadDockerClient(); err != nil {
				fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to download Docker client: %v\n", err)
			}
			if err := downloadDockerCompose(); err != nil {
				fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to download Docker Compose: %v\n", err)
			}
		}

		if os.Getenv("ZELLIJ_ENABLED") == "1" {
			if err := downloadZellij(); err != nil {
				fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to download Zellij: %v\n", err)
			}
		}

		if dockerGID := os.Getenv("DOCKER_GID"); dockerGID != "" {
			if err := setupDockerGroup(dockerGID); err != nil {
				fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to setup docker group: %v\n", err)
			}
		}

		os.MkdirAll("/home/kilo-t8x3m7kp/.local", 0755)
		os.MkdirAll("/workspace", 0755)

		if sshAuthSock := os.Getenv("SSH_AUTH_SOCK"); sshAuthSock != "" {
			if info, err := os.Stat(sshAuthSock); err == nil && info.Mode()&os.ModeSocket != 0 {
				os.Chown(sshAuthSock, puid, pgid)
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

		chownDirs := []string{
			"/home/kilo-t8x3m7kp/.ssh",
			"/home/kilo-t8x3m7kp/.config",
			"/home/kilo-t8x3m7kp/.local",
			"/workspace",
		}
		for _, dir := range chownDirs {
			chownRecursive(dir, puid, pgid)
		}

		syscall.Setgid(pgid)
		syscall.Setuid(puid)
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

	home := os.Getenv("HOME")
	if home == "" {
		home = "/home/kilo-t8x3m7kp"
	}

	if os.Getenv("ZELLIJ_ENABLED") == "1" {
		zellijConfigDir := filepath.Join(home, ".config", "zellij")
		os.MkdirAll(zellijConfigDir, 0755)
		zellijConfigPath := filepath.Join(zellijConfigDir, "config.kdl")
		if _, err := os.Stat(zellijConfigPath); os.IsNotExist(err) {
			src, err := os.Open("/etc/zellij/config.kdl")
			if err == nil {
				defer src.Close()
				dst, err := os.Create(zellijConfigPath)
				if err == nil {
					defer dst.Close()
					io.Copy(dst, src)
				}
			}
		}
	}

	binaryPath, _ := os.Executable()

	if len(os.Args) <= 1 {
		return syscall.Exec("/bin/sh", []string{"sh"}, os.Environ())
	}

	return syscall.Exec(binaryPath, os.Args[1:], os.Environ())
}

// downloadDockerClient fetches the latest Docker static binary from
// download.docker.com and installs it to /usr/local/bin/docker. Skips
// if docker is already on PATH.
func downloadDockerClient() error {
	if _, err := exec.LookPath("docker"); err == nil {
		return nil
	}
	fmt.Fprintf(os.Stderr, "[kilo-docker] Downloading latest Docker client...\n")
	resp, err := http.Get("https://download.docker.com/linux/static/stable/x86_64/")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	content := string(body)
	idx := strings.LastIndex(content, "docker-")
	if idx == -1 {
		return fmt.Errorf("could not find docker version")
	}
	rest := content[idx:]
	end := strings.Index(rest, ".tgz")
	if end == -1 {
		return fmt.Errorf("could not parse docker version")
	}
	version := strings.TrimPrefix(rest[:end], "docker-")

	url := fmt.Sprintf("https://download.docker.com/linux/static/stable/x86_64/docker-%s.tgz", version)
	resp2, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()

	tmpDir, _ := os.MkdirTemp("", "docker-download")
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, "docker.tgz")
	f, _ := os.Create(tarPath)
	io.Copy(f, resp2.Body)
	f.Close()

	cmd := exec.Command("tar", "xzf", tarPath, "-C", tmpDir, "docker/docker")
	cmd.Run()

	src, _ := os.Open(filepath.Join(tmpDir, "docker/docker"))
	defer src.Close()
	dst, _ := os.Create("/usr/local/bin/docker")
	defer dst.Close()
	io.Copy(dst, src)
	os.Chmod("/usr/local/bin/docker", 0755)
	return nil
}

// downloadDockerCompose fetches the latest Docker Compose binary from GitHub
// and installs it to /usr/local/bin/docker-compose and the CLI plugins directory.
func downloadDockerCompose() error {
	if _, err := exec.LookPath("docker-compose"); err == nil {
		return nil
	}
	fmt.Fprintf(os.Stderr, "[kilo-docker] Downloading latest Docker Compose...\n")
	resp, err := http.Get("https://github.com/docker/compose/releases/latest/download/docker-compose-linux-x86_64")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, _ := os.Create("/usr/local/bin/docker-compose")
	defer f.Close()
	io.Copy(f, resp.Body)
	os.Chmod("/usr/local/bin/docker-compose", 0755)

	os.MkdirAll("/usr/libexec/docker/cli-plugins", 0755)
	os.Symlink("/usr/local/bin/docker-compose", "/usr/libexec/docker/cli-plugins/docker-compose")
	return nil
}

// downloadZellij fetches the latest Zellij binary from GitHub and extracts
// it to /usr/local/bin.
func downloadZellij() error {
	if _, err := exec.LookPath("zellij"); err == nil {
		return nil
	}
	fmt.Fprintf(os.Stderr, "[kilo-docker] Downloading latest Zellij...\n")
	resp, err := http.Get("https://github.com/zellij-org/zellij/releases/latest/download/zellij-x86_64-unknown-linux-musl.tar.gz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	tmpDir, _ := os.MkdirTemp("", "zellij-download")
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, "zellij.tar.gz")
	f, _ := os.Create(tarPath)
	io.Copy(f, resp.Body)
	f.Close()

	cmd := exec.Command("tar", "xzf", tarPath, "-C", "/usr/local/bin")
	return cmd.Run()
}

// setupDockerGroup creates or joins the Docker group with the specified GID
// and adds the kilo user to it, enabling container-level Docker socket access.
func setupDockerGroup(gid string) error {
	cmd := exec.Command("addgroup", "-g", gid, "docker")
	if err := cmd.Run(); err == nil {
		exec.Command("addgroup", "kilo-t8x3m7kp", "docker").Run()
		return nil
	}
	cmd2 := exec.Command("getent", "group", gid)
	out, err := cmd2.Output()
	if err != nil {
		return nil
	}
	parts := strings.SplitN(string(out), ":", 2)
	if len(parts) > 0 && parts[0] != "" {
		exec.Command("addgroup", "kilo-t8x3m7kp", parts[0]).Run()
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
// It skips files already owned by the target user and uses fs.SkipDir
// to avoid recursing into directory trees that are fully owned by the
// target user. This avoids repeated chown on persistent volume data
// that was fixed on first run, while still catching Docker-build
// artifacts (COPYed files owned by root).
func chownRecursive(path string, uid, gid int) {
	filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
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
				// Already correct — skip file, or skip entire subtree for dirs
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			os.Chown(p, uid, gid)
		}
		return nil
	})
}
