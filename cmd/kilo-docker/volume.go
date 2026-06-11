package main

import (
	"crypto/sha256"
	"fmt"
	"os/exec"
)

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
