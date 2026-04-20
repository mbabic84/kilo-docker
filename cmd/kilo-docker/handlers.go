package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mbabic84/kilo-docker/pkg/utils"
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

// latestVersions holds the latest available versions from the remote.
type latestVersions struct {
	kiloDockerVersion string
	kiloVersion       string
}

// getLatestVersions fetches the latest versions from the .versions file.
func getLatestVersions() (latestVersions, error) {
	url := "https://github.com/mbabic84/kilo-docker/releases/latest/download/default.versions"
	resp, err := http.Get(url)
	if err != nil {
		return latestVersions{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return latestVersions{}, fmt.Errorf("failed to fetch versions: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return latestVersions{}, err
	}

	versions := latestVersions{}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "KILO_DOCKER_VERSION=") {
			versions.kiloDockerVersion = strings.TrimPrefix(line, "KILO_DOCKER_VERSION=")
		} else if strings.HasPrefix(line, "KILO_VERSION=") {
			versions.kiloVersion = strings.TrimPrefix(line, "KILO_VERSION=")
		}
	}
	return versions, nil
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
func handleUpdate(cfg config) {
	if cfg.help {
		if len(cfg.args) > 0 && cfg.args[0] == "config" {
			printCommandHelp("update config")
			return
		}
		printCommandHelp("update")
		return
	}

	if len(cfg.args) > 0 && cfg.args[0] == "config" {
		handleUpdateConfig(cfg)
		return
	}

	if len(cfg.args) > 0 && cfg.args[0] != "config" {
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\nRun 'kilo-docker update -h' for usage.\n", cfg.args[0])
		os.Exit(1)
	}

	home, _ := os.UserHomeDir()
	target := filepath.Join(home, ".local", "bin", "kilo-docker")

	latest, err := getLatestVersions()
	if err != nil {
		utils.LogWarn("[kilo-docker] Warning: could not fetch latest version info: %v\n", err)
		latest.kiloDockerVersion = "unknown"
		latest.kiloVersion = "unknown"
	}

	utils.Log("[kilo-docker] kilo-docker: %s → %s\n", version, latest.kiloDockerVersion, utils.WithOutput())
	utils.Log("[kilo-docker] Kilo CLI: %s → %s\n", kiloVersion, latest.kiloVersion, utils.WithOutput())

	if version == latest.kiloDockerVersion {
		utils.Log("[kilo-docker] \nAlready on latest version (%s). No update needed.\n", latest.kiloDockerVersion, utils.WithOutput())
		if !dockerDaemonRunning() {
			utils.LogWarn("\nWarning: Docker daemon is not running.\n")
		}
		return
	}

	if _, err := os.Stat(target); err != nil {
		utils.Log("[kilo-docker] kilo-docker is not installed locally.\n", utils.WithOutput())
		utils.Log("[kilo-docker] Run the install script first: curl -fsSL https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/install.sh | sh\n", utils.WithOutput())
	} else {
		osName, arch := getOSArch()
		downloadURL := fmt.Sprintf("https://github.com/mbabic84/kilo-docker/releases/latest/download/kilo-docker-%s-%s", osName, arch)

		utils.Log("[kilo-docker] \nUpdating kilo-docker binary...\n", utils.WithOutput())

		tempFile, err := os.CreateTemp("", "kilo-docker-*")
		if err != nil {
			utils.LogError("[kilo-docker] Error: failed to create temp file: %v\n", err)
		} else {
			tempPath := tempFile.Name()
			_ = tempFile.Close()

			if err := downloadFile(downloadURL, tempPath); err != nil {
				_ = os.Remove(tempPath)
				utils.LogError("[kilo-docker] Error: Failed to download update: %v\n", err)
			} else {
				if err := os.Chmod(tempPath, 0755); err != nil {
					_ = os.Remove(tempPath)
					utils.LogError("[kilo-docker] Error: failed to set permissions: %v\n", err)
				} else if err := os.Rename(tempPath, target); err != nil {
					_ = os.Remove(tempPath)
					utils.LogError("[kilo-docker] Error: failed to replace binary: %v\n", err)
				} else {
					utils.Log("[kilo-docker] Binary updated: %s\n", target, utils.WithOutput())
				}
			}
		}
	}

	if !dockerDaemonRunning() {
		utils.LogWarn("[kilo-docker] Warning: Docker daemon is not running. Skipping image pull.\n")
		utils.Log("[kilo-docker] Run 'docker pull %s:latest' after starting Docker.\n", repoURL, utils.WithOutput())
	} else {
		utils.Log("[kilo-docker] \nPulling Docker image...\n", utils.WithOutput())
		_, _ = dockerRun("pull", repoURL+":latest")
	}
	utils.Log("[kilo-docker] \nUpdated ✓\n", utils.WithOutput())
}

// handleCleanup removes all kilo-docker artifacts: containers, volumes,
// Docker images, and the installed script.
func handleCleanup(cfg config) {
	if cfg.help {
		printCommandHelp("cleanup")
		return
	}
	if !promptConfirm("Remove volume, containers, and images for kilo-docker? [y/N]: ", cfg.yes) {
		utils.Log("[kilo-docker] Aborted.\n", utils.WithOutput())
		return
	}

	output, _ := dockerRun("ps", "-a", "--filter", "ancestor="+repoURL, "-q")
		if output != "" {
		for _, id := range strings.Split(output, "\n") {
			if id != "" {
				_, _ = dockerRun("rm", "-f", id)
			}
		}
	}

	home, _ := os.UserHomeDir()
	user := filepath.Base(home)
	dataVolume := "kilo-data-" + user
	if volumeExists(dataVolume) {
		_ = removeVolume(dataVolume)
	}

	_, _ = dockerRun("rmi", repoURL+":latest")

	target := filepath.Join(home, ".local", "bin", "kilo-docker")
	_ = os.Remove(target)

	utils.Log("[kilo-docker] Cleanup complete.\n", utils.WithOutput())
}

// handleInit resets the data volume, prompting for confirmation.
func handleInit(cfg config) {
	if cfg.help {
		printCommandHelp("init")
		return
	}
	dataVolume := resolveVolume(cfg)
	if dataVolume == "" {
		utils.Log("[kilo-docker] Nothing to reset in --once mode.\n", utils.WithOutput())
		os.Exit(0)
	}

	if volumeExists(dataVolume) {
		if promptConfirm("Remove volume '" + dataVolume + "' and reset all configuration? [y/N]: ", cfg.yes) {
			_ = removeVolume(dataVolume)
			utils.Log("[kilo-docker] Volume removed. You will be prompted for tokens on next run.\n", utils.WithOutput())
		}
	} else {
		utils.Log("[kilo-docker] No existing volume found.\n", utils.WithOutput())
	}
}

// handleUpdateConfig downloads the latest opencode.json template and
// merges it with the existing config in the volume.
func handleUpdateConfig(cfg config) {
	if cfg.help {
		printCommandHelp("update config")
		return
	}
	dataVolume := resolveVolume(cfg)
	if dataVolume == "" {
		utils.Log("[kilo-docker] Nothing to update in --once mode.\n", utils.WithOutput())
		os.Exit(0)
	}

	if !volumeExists(dataVolume) {
		utils.Log("[kilo-docker] No existing volume found. Run kilo-docker first to create one.\n", utils.WithOutput())
		os.Exit(1)
	}

	utils.Log("[kilo-docker] Updating opencode.json from repository template...\n", utils.WithOutput())

	_, err := dockerRun(
		"-v", dataVolume+":"+kiloHome,
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		repoURL+":latest",
		"update-config",
	)
	if err != nil {
		utils.LogError("[kilo-docker] Error: %v\n", err)
		os.Exit(1)
	}
}
