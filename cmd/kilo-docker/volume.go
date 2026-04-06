package main

import (
	"crypto/sha256"
	"fmt"
	"os/exec"
)

// deriveContainerName returns the Docker container name for a given working
// directory. The name is derived from the SHA-256 hash (first 6 bytes) to
// ensure one session per directory.
func deriveContainerName(pwd string) string {
	hash := sha256.Sum256([]byte(pwd))
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
