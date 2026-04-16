package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

const (
	SharedNetworkName           = "kilo-shared"
	PlaywrightVolumeName       = "kilo-playwright-output"
	PlaywrightMountPath        = "/home/node"
	SharedPlaywrightContainerName = "kilo-playwright-mcp"
)

// normalizeNetworks ensures kilo-shared is always present if includeShared is true,
// removes duplicates, and returns a deterministic order (shared first, then user-provided).
func normalizeNetworks(networks []string, includeShared bool) []string {
	seen := make(map[string]bool)
	var result []string

	// Always include shared network first if requested
	if includeShared && SharedNetworkName != "" {
		result = append(result, SharedNetworkName)
		seen[SharedNetworkName] = true
	}

	// Add user-provided networks in order, skipping duplicates
	for _, n := range networks {
		if n != "" && !seen[n] {
			result = append(result, n)
			seen[n] = true
		}
	}

	return result
}

// EnsureSharedNetwork creates the shared network if it doesn't exist.
func EnsureSharedNetwork() error {
	if _, err := dockerRun("network", "inspect", SharedNetworkName); err == nil {
		return nil
	}

	output, err := dockerRun("network", "create", SharedNetworkName)
	if err != nil {
		return fmt.Errorf("failed to create shared network %s: %w", SharedNetworkName, err)
	}
	utils.LogWarn("[network] Created shared network: %s\n", SharedNetworkName)
	_ = output
	return nil
}

// EnsurePlaywrightVolume creates the Playwright output volume if it doesn't exist.
func EnsurePlaywrightVolume() error {
	if _, err := dockerRun("volume", "inspect", PlaywrightVolumeName); err == nil {
		return nil
	}

	output, err := dockerRun("volume", "create", PlaywrightVolumeName)
	if err != nil {
		return fmt.Errorf("failed to create Playwright volume %s: %w", PlaywrightVolumeName, err)
	}
	utils.LogWarn("[network] Created Playwright volume: %s\n", PlaywrightVolumeName)
	_ = output
	return nil
}

// selectNetwork displays an interactive list of available Docker networks
// and returns the user's selection. Returns empty string for the default network.
func selectNetwork() (string, error) {
	cmd := exec.Command("docker", "network", "ls", "--format", "{{.Name}}")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	networks := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(networks) == 0 {
		return "", nil
	}

	fmt.Fprintf(os.Stderr, "Available Docker networks:\n")
	for i, n := range networks {
		fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, n)
	}
	fmt.Fprintf(os.Stderr, "Select a network (number) or press Enter to use default: ")

	var selection string
	_, _ = fmt.Scanln(&selection)
	selection = strings.TrimSpace(selection)

	if selection == "" {
		return "", nil
	}

	var idx int
	_, _ = fmt.Sscanf(selection, "%d", &idx)
	if idx >= 1 && idx <= len(networks) {
		return networks[idx-1], nil
	}

	fmt.Fprintf(os.Stderr, "Invalid selection. Using default network.\n")
	return "", nil
}

// listNetworks prints all Docker network names to stdout.
func listNetworks() error {
	cmd := exec.Command("docker", "network", "ls", "--format", "{{.Name}}")
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
