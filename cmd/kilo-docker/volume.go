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

// createVolume creates a new Docker volume with the given name.
func createVolume(name string) error {
	_, err := exec.Command("docker", "volume", "create", name).CombinedOutput()
	return err
}

// removeVolume deletes a Docker volume. Returns an error if the volume
// is in use or doesn't exist.
func removeVolume(name string) error {
	_, err := exec.Command("docker", "volume", "rm", name).CombinedOutput()
	return err
}

// listVolumes returns all Docker volume names.
func listVolumes() ([]string, error) {
	cmd := exec.Command("docker", "volume", "ls", "--format", "{{.Name}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var volumes []string
	for _, line := range splitLines(string(output)) {
		if line != "" {
			volumes = append(volumes, line)
		}
	}
	return volumes, nil
}

// splitLines splits a string into lines without allocating a new slice
// for trailing empty lines.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
