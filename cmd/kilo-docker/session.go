package main

import (
	"fmt"
	"os"
	"strings"
)

// session represents a running or stopped kilo-docker container with its metadata.
type session struct {
	Name      string
	Status    string
	Workspace string
	Args      string
	User      string
}

// getSessions queries Docker for all containers labeled with kilo.workspace.
func getSessions() ([]session, error) {
	output, err := dockerRun("ps", "-a", "--filter", "label=kilo.workspace", "--format", "{{.Names}}\t{{.Status}}\t{{.Labels}}")
	if err != nil {
		return nil, err
	}

	var sessions []session
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		s := session{
			Name:   parts[0],
			Status: parts[1],
		}
		for _, label := range strings.Split(parts[2], ",") {
			if strings.HasPrefix(label, "kilo.workspace=") {
				s.Workspace = strings.TrimPrefix(label, "kilo.workspace=")
			}
			if strings.HasPrefix(label, "kilo.args=") {
				// Parse stored args to get network config for display
				storedArgs := strings.TrimPrefix(label, "kilo.args=")
				cfg := parseArgs(strings.Fields(storedArgs))
				s.Args = serializeForDisplay(cfg, false)
			}
			if strings.HasPrefix(label, "kilo.owner=") {
				s.User = strings.TrimPrefix(label, "kilo.owner=")
			}
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// showSessions prints a formatted table of sessions to stderr.
func showSessions(sessions []session) {
	fmt.Fprintf(os.Stderr, "%-4s %-22s %-32s %-52s %s\n", "#", "NAME", "STATUS", "WORKSPACE", "ARGS")
	fmt.Fprintf(os.Stderr, "%-4s %-22s %-32s %-52s %s\n", "---", "----------------------", "--------------------------------", "----------------------------------------------------", "-----")
	for i, s := range sessions {
		fmt.Fprintf(os.Stderr, "%-4d %-22s %-32s %-52s %s\n", i+1, s.Name, s.Status, s.Workspace, s.Args)
	}
}

// resolveTarget converts a target (numeric index or container name) to a
// container name. Returns an error if the container doesn't exist or the
// index is out of range.
func resolveTarget(target string) (string, error) {
	for _, c := range target {
		if c < '0' || c > '9' {
			if containerExists(target) {
				return target, nil
			}
			return "", fmt.Errorf("container '%s' not found", target)
		}
	}

	sessions, err := getSessions()
	if err != nil {
		return "", err
	}

	idx := 0
	_, _ = fmt.Sscanf(target, "%d", &idx)
	if idx < 1 || idx > len(sessions) {
		return "", fmt.Errorf("no session at index %s", target)
	}
	return sessions[idx-1].Name, nil
}
