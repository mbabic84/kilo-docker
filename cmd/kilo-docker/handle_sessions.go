package main

import (
	"fmt"
	"os"
)

// handleSessions lists, attaches to, or cleans up kilo-docker sessions.
func handleSessions(cfg config) {
	args := cfg.args
	cleanupMode := false
	cleanupYes := false
	cleanupAll := false
	attachTarget := ""

	if len(args) > 0 && args[0] == "cleanup" {
		cleanupMode = true
		args = args[1:]
		for len(args) > 0 && (args[0] == "-y" || args[0] == "--yes" || args[0] == "-a" || args[0] == "--all") {
			switch args[0] {
			case "-y", "--yes":
				cleanupYes = true
			case "-a", "--all":
				cleanupAll = true
			}
			args = args[1:]
		}
	}

	if len(args) > 0 {
		attachTarget = args[0]
	}

	sessions, err := getSessions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if cleanupMode {
		if cleanupAll {
			removed := 0
			for _, s := range sessions {
				if dockerState(s.Name) == "exited" {
					dockerRun("rm", "-f", s.Name)
					fmt.Fprintf(os.Stderr, "Session '%s' removed.\n", s.Name)
					removed++
				}
			}
			if removed == 0 {
				fmt.Fprintf(os.Stderr, "No exited sessions to clean up.\n")
			}
			return
		}

		containerToClean := ""
		if attachTarget != "" {
			containerToClean, err = resolveTarget(attachTarget)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			if len(sessions) == 0 {
				fmt.Fprintf(os.Stderr, "No sessions to clean up.\n")
				os.Exit(0)
			}
			showSessions(sessions)
			fmt.Print("Select session to remove (number or name): ")
			var selection string
			fmt.Scanln(&selection)
			if selection == "" {
				fmt.Fprintf(os.Stderr, "Aborted.\n")
				os.Exit(0)
			}
			containerToClean, err = resolveTarget(selection)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}

		if cleanupYes || promptConfirm("Remove session '"+containerToClean+"'? [y/N]: ") {
			dockerRun("rm", "-f", containerToClean)
			fmt.Fprintf(os.Stderr, "Session '%s' removed.\n", containerToClean)
		} else {
			fmt.Fprintf(os.Stderr, "Aborted.\n")
		}
		return
	}

	if attachTarget == "" {
		if len(sessions) == 0 {
			fmt.Fprintf(os.Stderr, "No active sessions.\n")
			os.Exit(0)
		}
		showSessions(sessions)
		return
	}

	containerToAttach, err := resolveTarget(attachTarget)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	state := dockerState(containerToAttach)
	switch state {
	case "running":
		fmt.Fprintf(os.Stderr, "Attaching to running session '%s' (detach: Ctrl+P Ctrl+Q)...\n", containerToAttach)
		execDockerAttach("attach", containerToAttach)
		resetTerminal()
	case "exited", "created":
		fmt.Fprintf(os.Stderr, "Starting session '%s'...\n", containerToAttach)
		execDockerAttach("start", "-ai", containerToAttach)
		resetTerminal()
	default:
		fmt.Fprintf(os.Stderr, "Error: Container '%s' is in state '%s'.\n", containerToAttach, state)
		os.Exit(1)
	}
}
