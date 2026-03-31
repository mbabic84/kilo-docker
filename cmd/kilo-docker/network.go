package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

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
	fmt.Scanln(&selection)
	selection = strings.TrimSpace(selection)

	if selection == "" {
		return "", nil
	}

	var idx int
	fmt.Sscanf(selection, "%d", &idx)
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
