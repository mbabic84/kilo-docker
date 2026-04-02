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
		fmt.Fprintf(os.Stderr, "Run the install script first: curl -fsSL https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/install.sh | sh\n")
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
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		repoURL+":latest",
		"update-config",
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
