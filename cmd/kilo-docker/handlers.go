package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// downloadFile downloads a file from url to dest, trying curl first then wget.
func downloadFile(url, dest string) error {
	cmd := exec.Command("curl", "-fsSL", url, "-o", dest)
	if err := cmd.Run(); err == nil {
		return nil
	}
	cmd = exec.Command("wget", "-q", "-O", dest, url)
	if err := cmd.Run(); err == nil {
		return nil
	}
	return fmt.Errorf("neither curl nor wget succeeded")
}

// getOSArch returns the OS and architecture for the current system.
func getOSArch() (osName, arch string) {
	osName = runtime.GOOS
	arch = runtime.GOARCH
	switch osName {
	case "darwin", "linux":
		// valid
	default:
		osName = "linux"
	}
	switch arch {
	case "arm64", "aarch64":
		arch = "arm64"
	default: // amd64 and unknown default to amd64
	}
	return
}

// handleUpdate downloads the latest kilo-docker binary from GitHub releases
// and pulls the latest Docker image.
func handleUpdate() {
	home, _ := os.UserHomeDir()
	target := filepath.Join(home, ".local", "bin", "kilo-docker")

	// Check if installed
	if _, err := os.Stat(target); err != nil {
		fmt.Fprintf(os.Stderr, "kilo-docker is not installed locally.\n")
		fmt.Fprintf(os.Stderr, "Run 'kilo-docker install' to install it first.\n")
	} else {
		// Download latest binary from GitHub releases
		osName, arch := getOSArch()
		downloadURL := fmt.Sprintf("https://github.com/mbabic84/kilo-docker/releases/latest/download/kilo-docker-%s-%s", osName, arch)

		fmt.Fprintf(os.Stderr, "Updating kilo-docker binary...\n")

		tempFile, err := os.CreateTemp("", "kilo-docker-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create temp file: %v\n", err)
		} else {
			tempPath := tempFile.Name()
			tempFile.Close()

			if err := downloadFile(downloadURL, tempPath); err != nil {
				os.Remove(tempPath)
				fmt.Fprintf(os.Stderr, "Error: Failed to download update: %v\n", err)
			} else {
				if err := os.Chmod(tempPath, 0755); err != nil {
					os.Remove(tempPath)
					fmt.Fprintf(os.Stderr, "Error: failed to set permissions: %v\n", err)
				} else if err := os.Rename(tempPath, target); err != nil {
					os.Remove(tempPath)
					fmt.Fprintf(os.Stderr, "Error: failed to replace binary: %v\n", err)
				} else {
					fmt.Fprintf(os.Stderr, "Binary updated: %s\n", target)
				}
			}
		}
	}

	if !dockerDaemonRunning() {
		fmt.Fprintf(os.Stderr, "\nWarning: Docker daemon is not running. Skipping image pull.\n")
		fmt.Fprintf(os.Stderr, "Run 'docker pull %s:latest' after starting Docker.\n", repoURL)
	} else {
		fmt.Fprintf(os.Stderr, "\nPulling Docker image...\n")
		dockerRun("pull", repoURL+":latest")
	}
	fmt.Fprintf(os.Stderr, "\nUpdate complete.\n")
}

// handleInstall installs kilo-docker as a global command by creating a
// symlink (or copy) in ~/.local/bin and pulling the Docker image.
func handleInstall() {
	home, _ := os.UserHomeDir()
	target := filepath.Join(home, ".local", "bin", "kilo-docker")

	sourceScript, _ := os.Executable()
	os.MkdirAll(filepath.Dir(target), 0755)

	if info, err := os.Lstat(target); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			fmt.Fprintf(os.Stderr, "kilo-docker is already installed at %s\n", target)
			fmt.Fprintf(os.Stderr, "Updating symlink...\n")
			os.Remove(target)
		} else {
			fmt.Fprintf(os.Stderr, "kilo-docker already exists at %s\n", target)
			if !promptConfirm("Replace it? [y/N]: ") {
				fmt.Fprintf(os.Stderr, "Aborted.\n")
				return
			}
			os.Remove(target)
		}
	}

	if err := os.Symlink(sourceScript, target); err != nil {
		if err := copyFile(sourceScript, target); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to install binary: %v\n", err)
			os.Exit(1)
		}
	}
	os.Chmod(target, 0755)

	path := os.Getenv("PATH")
	if !strings.Contains(path, filepath.Join(home, ".local", "bin")) {
		fmt.Fprintf(os.Stderr, "\nWarning: %s is not on your PATH.\n", filepath.Join(home, ".local", "bin"))
		fmt.Fprintf(os.Stderr, "Add the following line to your ~/.bashrc (or ~/.zshrc):\n")
		fmt.Fprintf(os.Stderr, "  export PATH=\"$HOME/.local/bin:$PATH\"\n")
	}

	// Verify the binary runs before reporting success.
	if out, err := exec.Command(target, "--version").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "\nWarning: installed binary failed --version check: %v\n", err)
		if len(out) > 0 {
			fmt.Fprintf(os.Stderr, "  %s\n", strings.TrimSpace(string(out)))
		}
	}

	if !dockerDaemonRunning() {
		fmt.Fprintf(os.Stderr, "\nWarning: Docker daemon is not running. Skipping image pull.\n")
		fmt.Fprintf(os.Stderr, "Run 'docker pull %s:latest' after starting Docker.\n", repoURL)
	} else {
		fmt.Fprintf(os.Stderr, "\nPulling Docker image...\n")
		dockerRun("pull", repoURL+":latest")
	}

	fmt.Fprintf(os.Stderr, "\nInstalled successfully: %s -> %s\n", target, sourceScript)
	fmt.Fprintf(os.Stderr, "Run 'kilo-docker' from any directory.\n")
}

// handleCleanup removes all kilo-docker artifacts: containers, volumes,
// Docker images, and the installed script.
func handleCleanup() {
	if !promptConfirm("Remove volume, containers, and images for kilo-docker? [y/N]: ") {
		fmt.Fprintf(os.Stderr, "Aborted.\n")
		return
	}

	output, _ := dockerRun("ps", "-a", "--filter", "ancestor="+repoURL, "-q")
	if output != "" {
		for _, id := range strings.Split(output, "\n") {
			if id != "" {
				dockerRun("rm", "-f", id)
			}
		}
	}

	home, _ := os.UserHomeDir()
	user := filepath.Base(home)
	dataVolume := "kilo-data-" + user
	if volumeExists(dataVolume) {
		removeVolume(dataVolume)
	}

	dockerRun("rmi", repoURL+":latest")

	target := filepath.Join(home, ".local", "bin", "kilo-docker")
	os.Remove(target)

	fmt.Fprintf(os.Stderr, "Cleanup complete.\n")
}

// handleInit resets the data volume, prompting for confirmation.
func handleInit(cfg config) {
	dataVolume := resolveVolume(cfg)
	if dataVolume == "" {
		fmt.Fprintf(os.Stderr, "Nothing to reset in --once mode.\n")
		os.Exit(0)
	}

	if volumeExists(dataVolume) {
		if promptConfirm("Remove volume '" + dataVolume + "' and reset all configuration? [y/N]: ") {
			removeVolume(dataVolume)
			fmt.Fprintf(os.Stderr, "Volume removed. You will be prompted for tokens on next run.\n")
		}
	} else {
		fmt.Fprintf(os.Stderr, "No existing volume found.\n")
	}
}

// handleUpdateConfig downloads the latest opencode.json template and
// merges it with the existing config in the volume.
func handleUpdateConfig(cfg config) {
	dataVolume := resolveVolume(cfg)
	if dataVolume == "" {
		fmt.Fprintf(os.Stderr, "Nothing to update in --once mode.\n")
		os.Exit(0)
	}

	if !volumeExists(dataVolume) {
		fmt.Fprintf(os.Stderr, "No existing volume found. Run kilo-docker first to create one.\n")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Updating opencode.json from repository template...\n")

	_, err := dockerRun(
		"-v", dataVolume+":"+kiloHome,
		repoURL+":latest",
		"update-config",
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
