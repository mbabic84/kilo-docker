package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

// downloadFile downloads a file from url to dest.
func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = io.Copy(f, resp.Body)
	return err
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
				if err := os.Chmod(tempPath, 0o755); err != nil {
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
// Docker images, and the installed script. Each destructive step requires
// explicit user confirmation. The -y/--yes flag is intentionally ignored.
func handleCleanup(cfg config) {
	if cfg.help {
		printCommandHelp("cleanup")
		return
	}

	home, _ := os.UserHomeDir()
	_, username := resolveWorkspaceAndUsername()
	dataVolume := resolveVolume(cfg, username)
	binaryPath := filepath.Join(home, ".local", "bin", "kilo-docker")
	sessions, _ := getSessions()
	volExists := dataVolume != "" && volumeExists(dataVolume)
	imgExists := imageExists(repoURL + ":latest")
	binExists := fileExists(binaryPath)

	showCleanupSummary(sessions, dataVolume, volExists, imgExists, binExists, binaryPath)

	if !promptConfirmStrict("Do you understand and want to proceed? [y/N]: ") {
		utils.Log("[kilo-docker] Aborted.\n", utils.WithOutput())
		return
	}
	utils.Log("[kilo-docker] \n", utils.WithOutput())

	containersSkipped := cleanupStepContainers()

	utils.Log("[kilo-docker] \n", utils.WithOutput())
	cleanupStepVolume(dataVolume, volExists, containersSkipped)

	utils.Log("[kilo-docker] \n", utils.WithOutput())
	cleanupStepImage(imgExists)

	utils.Log("[kilo-docker] \n", utils.WithOutput())
	cleanupStepBinary(binaryPath, binExists)

	utils.Log("[kilo-docker] \nCleanup finished.\n", utils.WithOutput())
}

func showCleanupSummary(sessions []session, dataVolume string, volExists, imgExists, binExists bool, binaryPath string) {
	var running, exited []session
	for _, s := range sessions {
		if isContainerRunning(s.Name) {
			running = append(running, s)
		} else {
			exited = append(exited, s)
		}
	}

	utils.LogWarn("[kilo-docker] WARNING: This command will permanently destroy kilo-docker data.\n", utils.WithOutput())
	utils.LogWarn("[kilo-docker] You will be asked to confirm EACH step before anything is removed.\n\n", utils.WithOutput())

	utils.Log("[kilo-docker] Current state:\n", utils.WithOutput())
	if len(running) > 0 {
		utils.Log("[kilo-docker]   Running sessions:  %d\n", len(running), utils.WithOutput())
		for _, s := range running {
			utils.Log("[kilo-docker]     - %s  (%s)\n", s.Name, s.Workspace, utils.WithOutput())
		}
	}
	if len(exited) > 0 {
		utils.Log("[kilo-docker]   Exited sessions:   %d\n", len(exited), utils.WithOutput())
		for _, s := range exited {
			utils.Log("[kilo-docker]     - %s  (%s)\n", s.Name, s.Workspace, utils.WithOutput())
		}
	}
	if len(sessions) == 0 {
		utils.Log("[kilo-docker]   Sessions:          none\n", utils.WithOutput())
	}
	if volExists {
		utils.Log("[kilo-docker]   Data volume:       %s\n", dataVolume, utils.WithOutput())
	} else {
		utils.Log("[kilo-docker]   Data volume:       none\n", utils.WithOutput())
	}
	if imgExists {
		utils.Log("[kilo-docker]   Docker image:      %s:latest\n", repoURL, utils.WithOutput())
	} else {
		utils.Log("[kilo-docker]   Docker image:      not found\n", utils.WithOutput())
	}
	if binExists {
		utils.Log("[kilo-docker]   Binary:            %s\n", binaryPath, utils.WithOutput())
	} else {
		utils.Log("[kilo-docker]   Binary:            not found\n", utils.WithOutput())
	}
	utils.Log("[kilo-docker] \n", utils.WithOutput())
}

func cleanupStepContainers() bool {
	output, _ := dockerRun("ps", "-a", "--filter", "ancestor="+repoURL, "-q")
	ids := filterNonEmpty(strings.Split(output, "\n"))
	if len(ids) == 0 {
		utils.Log("[kilo-docker] No kilo-docker containers found.\n", utils.WithOutput())
		return false
	}

	utils.LogWarn("[kilo-docker] Found %d kilo-docker container(s) that will be FORCE-REMOVED:\n", len(ids), utils.WithOutput())
	for _, id := range ids {
		utils.LogWarn("[kilo-docker]   - %s\n", id, utils.WithOutput())
	}
	if !promptConfirmStrict("Remove these containers? [y/N]: ") {
		utils.Log("[kilo-docker] Skipping container removal.\n", utils.WithOutput())
		return true
	}
	for _, id := range ids {
		_, _ = dockerRun("rm", "-f", id)
	}
	utils.Log("[kilo-docker] Containers removed.\n", utils.WithOutput())
	return false
}

func cleanupStepVolume(dataVolume string, exists, containersSkipped bool) {
	if !exists {
		utils.Log("[kilo-docker] No data volume found.\n", utils.WithOutput())
		return
	}

	utils.LogWarn("[kilo-docker] DATA VOLUME '%s' will be PERMANENTLY DELETED.\n", dataVolume, utils.WithOutput())
	utils.LogWarn("[kilo-docker] This includes ALL sessions, configuration, and cached data.\n", utils.WithOutput())
	if containersSkipped {
		utils.LogWarn("[kilo-docker] NOTE: Container removal was skipped. The volume may still be in use.\n", utils.WithOutput())
		utils.LogWarn("[kilo-docker] Docker will refuse to delete a volume mounted by a running container.\n", utils.WithOutput())
	}
	if !promptConfirmStrict("Delete this volume? [y/N]: ") {
		utils.Log("[kilo-docker] Skipping volume deletion.\n", utils.WithOutput())
		return
	}
	if err := removeVolume(dataVolume); err != nil {
		utils.LogError("[kilo-docker] Failed to delete volume '%s': %v\n", dataVolume, err, utils.WithOutput())
		if containersSkipped {
			utils.LogError("[kilo-docker] The volume is likely still mounted by a running container.\n", utils.WithOutput())
			utils.LogError("[kilo-docker] Remove the containers first, then retry cleanup.\n", utils.WithOutput())
		}
		return
	}
	utils.Log("[kilo-docker] Volume deleted.\n", utils.WithOutput())
}

func cleanupStepImage(exists bool) {
	if !exists {
		utils.Log("[kilo-docker] Docker image not found, skipping.\n", utils.WithOutput())
		return
	}
	utils.LogWarn("[kilo-docker] Docker image '%s:latest' will be REMOVED.\n", repoURL, utils.WithOutput())
	if !promptConfirmStrict("Remove Docker image? [y/N]: ") {
		utils.Log("[kilo-docker] Skipping image removal.\n", utils.WithOutput())
		return
	}
	_, _ = dockerRun("rmi", repoURL+":latest")
	utils.Log("[kilo-docker] Image removed.\n", utils.WithOutput())
}

func cleanupStepBinary(path string, exists bool) {
	if !exists {
		utils.Log("[kilo-docker] No binary found at %s.\n", path, utils.WithOutput())
		return
	}
	utils.LogWarn("[kilo-docker] Binary '%s' will be DELETED.\n", path, utils.WithOutput())
	if !promptConfirmStrict("Delete binary? [y/N]: ") {
		utils.Log("[kilo-docker] Skipping binary deletion.\n", utils.WithOutput())
		return
	}
	_ = os.Remove(path)
	utils.Log("[kilo-docker] Binary deleted.\n", utils.WithOutput())
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func filterNonEmpty(items []string) []string {
	var result []string
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			result = append(result, item)
		}
	}
	return result
}

// handleInit resets the data volume, prompting for confirmation.
func handleInit(cfg config) {
	if cfg.help {
		printCommandHelp("init")
		return
	}
	_, username := resolveWorkspaceAndUsername()
	dataVolume := resolveVolume(cfg, username)
	if dataVolume == "" {
		utils.Log("[kilo-docker] Nothing to reset in --once mode.\n", utils.WithOutput())
		os.Exit(0)
	}

	if volumeExists(dataVolume) {
		if promptConfirm("Remove volume '"+dataVolume+"' and reset all configuration? [y/N]: ", cfg.yes) {
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
	_, username := resolveWorkspaceAndUsername()
	dataVolume := resolveVolume(cfg, username)
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
