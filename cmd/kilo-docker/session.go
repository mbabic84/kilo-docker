package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// session represents a running or stopped kilo-docker container with its metadata.
type session struct {
	Name             string
	Status           string
	Workspace        string
	Args             string
	User             string
	ImageVersion     string
	UsesLegacyVolume bool
	NeedsUpdate      bool
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
			if strings.HasPrefix(label, "kilo.version=") {
				s.ImageVersion = strings.TrimPrefix(label, "kilo.version=")
			}
		}
		sessions = append(sessions, s)
	}

	_, currentUsername := resolveWorkspaceAndUsername()
	expectedVolume := deriveVolumeName(currentUsername)
	for i := range sessions {
		sessions[i].UsesLegacyVolume = containerUsesLegacyVolume(sessions[i].Name, expectedVolume)
		sessions[i].NeedsUpdate = sessions[i].ImageVersion == "" || sessions[i].ImageVersion != version
	}

	return sessions, nil
}

// showSessions prints a formatted table of sessions to stderr.
func showSessions(sessions []session) {
	fmt.Fprintf(os.Stderr, "%-4s %-22s %-32s %-52s %s\n", "#", "NAME", "STATUS", "WORKSPACE", "ARGS")
	fmt.Fprintf(os.Stderr, "%-4s %-22s %-32s %-52s %s\n", "---", "----------------------", "--------------------------------", "----------------------------------------------------", "-----")
	for i, s := range sessions {
		status := s.Status
		if s.UsesLegacyVolume {
			status += " (legacy)"
		}
		if s.NeedsUpdate {
			status += " (update)"
		}
		fmt.Fprintf(os.Stderr, "%-4d %-22s %-32s %-52s %s\n", i+1, s.Name, status, s.Workspace, s.Args)
	}
}

// resolveTarget converts a target (numeric index, container name, or workspace
// name) to a container name. Resolution order:
//  1. Numeric index (all digits) → index lookup
//  2. Exact container name → return as-is
//  3. Workspace basename (exact match) → return that session
//  4. Workspace basename prefix match → return if unique, error if ambiguous
//  5. No match → error
func resolveTarget(target string) (string, error) {
	sessions, err := getSessions()
	if err != nil {
		return "", err
	}
	return resolveTargetWithSessions(target, sessions)
}

// resolveTargetWithSessions is like resolveTarget but accepts a pre-fetched
// sessions list to avoid redundant Docker calls.
func resolveTargetWithSessions(target string, sessions []session) (string, error) {
	for _, c := range target {
		if c < '0' || c > '9' {
			// Try workspace matching first — workspace basenames are the
			// primary user-facing identifier (used by tab completion).
			var matches []session
			for _, s := range sessions {
				base := filepath.Base(s.Workspace)
				if base == target || strings.HasPrefix(base, target) {
					matches = append(matches, s)
				}
			}

			switch len(matches) {
			case 1:
				return matches[0].Name, nil
			case 2, 3, 4, 5, 6, 7, 8, 9:
				var candidates []string
				for i, m := range matches {
					candidates = append(candidates, fmt.Sprintf("  %d) %s (%s)", i+1, filepath.Base(m.Workspace), m.Name))
				}
				return "", fmt.Errorf("ambiguous target '%s', did you mean:\n%s", target, strings.Join(candidates, "\n"))
			}

			// No workspace match — fall back to exact container name.
			if containerExists(target) {
				return target, nil
			}

			if len(matches) == 0 {
				return "", fmt.Errorf("no session matches '%s'", target)
			}
		}
	}

	idx := 0
	_, _ = fmt.Sscanf(target, "%d", &idx)
	if idx < 1 || idx > len(sessions) {
		return "", fmt.Errorf("no session at index %s", target)
	}
	return sessions[idx-1].Name, nil
}

// filterSessions returns sessions matching the given criteria.
// If legacy is true, only sessions using legacy volumes are included.
// If needsUpdate is true, only sessions needing an image update are included.
// If both are true, sessions matching either criterion are included.
func filterSessions(sessions []session, legacy, needsUpdate bool) []session {
	if !legacy && !needsUpdate {
		return sessions
	}
	var filtered []session
	for _, s := range sessions {
		if legacy && s.UsesLegacyVolume {
			filtered = append(filtered, s)
			continue
		}
		if needsUpdate && s.NeedsUpdate {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// showSessionCompletions prints tab-completion candidates to stdout, one per
// line. Each session produces two candidates: workspace basename and full
// workspace path.
func showSessionCompletions() {
	sessions, err := getSessions()
	if err != nil {
		return
	}
	for _, s := range sessions {
		fmt.Printf("%s\n", filepath.Base(s.Workspace))
		fmt.Printf("%s\n", s.Workspace)
	}
}
