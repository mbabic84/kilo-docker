package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"time"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

const legacyVolumeName = "kilo-docker-data"

// deriveContainerName returns the Docker container name for a given working
// directory and username. The name is derived from the SHA-256 hash (first 6 bytes)
// of "workspace:username" to ensure one session per user per directory.
// Both workspace and username are used so that two different users working in
// the same directory get distinct container names.
func deriveContainerName(pwd, username string) string {
	hash := sha256.Sum256([]byte(pwd + ":" + username))
	return fmt.Sprintf("kilo-%x", hash[:6])
}

// volumeExists reports whether a Docker volume with the given name exists.
func volumeExists(name string) bool {
	_, err := exec.Command("docker", "volume", "inspect", name).CombinedOutput()
	return err == nil
}

// removeVolume deletes a Docker volume. Returns an error if the volume
// is in use or doesn't exist.
func removeVolume(name string) error {
	_, err := exec.Command("docker", "volume", "rm", name).CombinedOutput()
	return err
}

// resolveWorkspaceAndUsername returns the current working directory (absolute)
// and the current OS username. Used by handler functions that need workspace/username
// but aren't called from runContainer (which already resolves both).
func resolveWorkspaceAndUsername() (string, string) {
	workspace, _ := os.Getwd()
	workspace, _ = filepath.Abs(workspace)
	u, _ := user.Current()
	username := "unknown"
	if u != nil {
		username = u.Username
	}
	return workspace, username
}

// migrateVolumeIfNeeded copies data from the legacy shared volume to the new
// per-user volume on first access. The legacy volume is left intact so the user
// can verify the migration succeeded before manually removing it.
func migrateVolumeIfNeeded(newVolume string) {
	if volumeExists(newVolume) {
		return
	}
	if !volumeExists(legacyVolumeName) {
		return
	}

	utils.Log("[kilo-docker] First-time migration: moving your data to a personal volume.\n", utils.WithOutput())
	utils.Log("[kilo-docker]   Source:      %s (shared, legacy)\n", legacyVolumeName, utils.WithOutput())
	utils.Log("[kilo-docker]   Destination: %s (personal)\n", newVolume, utils.WithOutput())
	utils.Log("[kilo-docker] This only happens once. Your data on the old volume is not deleted.\n\n", utils.WithOutput())

	utils.Log("[kilo-docker] Creating new volume '%s'...\n", newVolume, utils.WithOutput())
	if _, err := dockerRun("volume", "create", newVolume); err != nil {
		utils.LogError("[kilo-docker] Migration failed: could not create volume: %v\n", err)
		utils.LogError("[kilo-docker] You can retry by running 'kilo-docker' again, or manually create the volume.\n")
		os.Exit(1)
	}

	tempContainer := fmt.Sprintf("kilo-migrate-temp-%d", os.Getpid())
	utils.Log("[kilo-docker] Copying data...\n", utils.WithOutput())
	if _, err := dockerRun("run", "--rm", "-d", "--name", tempContainer,
		"-v", legacyVolumeName+":/src:ro",
		"-v", newVolume+":/dest",
		"debian:bookworm-slim", "tail", "-f", "/dev/null"); err != nil {
		utils.LogError("[kilo-docker] Migration failed: could not start copy container: %v\n", err)
		os.Exit(1)
	}
	time.Sleep(500 * time.Millisecond)
	if _, err := dockerRun("exec", tempContainer, "sh", "-c", "cp -a /src/. /dest/"); err != nil {
		utils.LogError("[kilo-docker] Migration failed during data copy: %v\n", err)
		utils.LogError("[kilo-docker] Cleaning up temporary container...\n")
		_, _ = dockerRun("rm", "-f", tempContainer)
		os.Exit(1)
	}
	_, _ = dockerRun("rm", "-f", tempContainer)

	utils.Log("[kilo-docker] Migration complete. Your personal volume '%s' is ready.\n", newVolume, utils.WithOutput())
	utils.Log("[kilo-docker] The legacy volume '%s' was left intact. Remove it manually with:\n", legacyVolumeName, utils.WithOutput())
	utils.Log("[kilo-docker]   docker volume rm %s\n\n", legacyVolumeName, utils.WithOutput())
}
