package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

// handleSessions lists, attaches to, recreates, or cleans up kilo-docker sessions.
func handleSessions(cfg config) {
	args := cfg.args
	cleanupMode := false
	cleanupYes := false
	cleanupAll := false
	recreateMode := false
	attachTarget := ""

	if cfg.help {
		if len(args) > 0 && args[0] == "cleanup" {
			printCommandHelp("sessions cleanup")
			return
		}
		if len(args) > 0 && args[0] == "recreate" {
			printCommandHelp("sessions recreate")
			return
		}
		printCommandHelp("sessions")
		return
	}

	if len(args) > 0 && args[0] == "cleanup" {
		cleanupMode = true
		args = args[1:]
	parseCleanupFlags:
		for len(args) > 0 {
			switch args[0] {
			case "-y", "--yes":
				cleanupYes = true
			case "-a", "--all":
				cleanupAll = true
			default:
				break parseCleanupFlags
			}
			args = args[1:]
		}
	}

	if len(args) > 0 && args[0] == "recreate" {
		recreateMode = true
		args = args[1:]
	}

	if len(args) > 0 {
		attachTarget = args[0]
	}

	sessions, err := getSessions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if recreateMode {
		if attachTarget == "" {
			fmt.Fprintf(os.Stderr, "Error: specify a session to recreate (name or index)\n")
			os.Exit(1)
		}
		containerName, err := resolveTarget(attachTarget)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		var targetSession session
		for _, s := range sessions {
			if s.Name == containerName {
				targetSession = s
				break
			}
		}

		if targetSession.Name == "" {
			fmt.Fprintf(os.Stderr, "Error: session '%s' not found\n", containerName)
			os.Exit(1)
		}

		// --once sessions have no volume to preserve
		if strings.Contains(targetSession.Args, "--once") {
			fmt.Fprintf(os.Stderr, "Error: Cannot recreate a --once session (no persistent volume)\n")
			os.Exit(1)
		}

		// Parse stored args back into a config.
		storedArgs := targetSession.Args
		parsedArgs := strings.Fields(storedArgs)
		utils.Log("[kilo-docker] Recreating with stored args: %q, parsed: %v\n", storedArgs, parsedArgs)

		newCfg := parseArgs(parsedArgs)
		utils.Log("[kilo-docker] Parsed config: remember=%v, ssh=%v, yes=%v, services=%v\n", newCfg.remember, newCfg.ssh, newCfg.yes, newCfg.enabledServices)
		newCfg.command = "" // ensure runContainer creates a new container
		utils.Log("[kilo-docker] Recreate config after normalization: yes=%v, serialized=%q\n", newCfg.yes, serializeArgs(newCfg, newCfg.ssh))

		// Change to the session's original workspace
		if targetSession.Workspace != "" {
			originalDir, _ := os.Getwd()
			if err := os.Chdir(targetSession.Workspace); err != nil {
				fmt.Fprintf(os.Stderr, "Error: cannot change to workspace '%s': %v\n", targetSession.Workspace, err)
				fmt.Fprintf(os.Stderr, "Current directory: %s\n", originalDir)
				os.Exit(1)
			}
			utils.Log("[kilo-docker] Recreating session in workspace: %s\n", targetSession.Workspace, utils.WithOutput())
		}

		// Remove the old container (volume persists)
		utils.Log("[kilo-docker] Removing old container '%s'...\n", containerName, utils.WithOutput())
		_, _ = dockerRun("rm", "-f", containerName)

		// Run with the original flags — this creates a fresh container
		// attached to the same volume (user data preserved).
		runContainer(newCfg)
		return
	}

	if cleanupMode {
		if cleanupAll {
			for _, s := range sessions {
				if dockerState(s.Name) != "exited" {
					continue
				}
				if cleanupYes || promptConfirm("Remove session '"+s.Name+"'? [y/N]: ", cleanupYes) {
					_, _ = dockerRun("rm", "-f", s.Name)
					fmt.Fprintf(os.Stderr, "Session '%s' removed.\n", s.Name)
				} else {
					fmt.Fprintf(os.Stderr, "Skipped '%s'.\n", s.Name)
				}
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
			_, _ = fmt.Scanln(&selection)
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

		if cleanupYes || promptConfirm("Remove session '"+containerToClean+"'? [y/N]: ", cleanupYes) {
			_, _ = dockerRun("rm", "-f", containerToClean)
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
	rememberFlag := ""
	for _, s := range sessions {
		if s.Name == containerToAttach {
			if strings.Contains(s.Args, "--remember") {
				rememberFlag = "--remember"
			}
			break
		}
	}
	switch state {
	case "running":
		// Pass --remember BEFORE zellij-attach so flag.Parse() catches it
		if rememberFlag != "" {
			_ = execDockerInteractive(containerToAttach, "kilo-entrypoint", rememberFlag, "zellij-attach")
		} else {
			_ = execDockerInteractive(containerToAttach, "kilo-entrypoint", "zellij-attach")
		}
		handleSessionEnd(containerToAttach, false)
	case "exited", "created":
		needsSSH := false
		for _, s := range sessions {
			if s.Name == containerToAttach && strings.Contains(s.Args, "--ssh") {
				needsSSH = true
				break
			}
		}
		if needsSSH {
			_, _, sshStarted := setupSSH()
			if sshStarted {
				defer cleanupSSH(os.Getenv("SSH_AGENT_PID"))
			}
		}
		_, _ = dockerRun("start", "-d", containerToAttach)
		time.Sleep(2 * time.Second)
		// Pass --remember BEFORE zellij-attach so flag.Parse() catches it
		if rememberFlag != "" {
			_ = execDockerInteractive(containerToAttach, "kilo-entrypoint", rememberFlag, "zellij-attach")
		} else {
			_ = execDockerInteractive(containerToAttach, "kilo-entrypoint", "zellij-attach")
		}
		handleSessionEnd(containerToAttach, false)
	default:
		fmt.Fprintf(os.Stderr, "Error: Container '%s' is in state '%s'.\n", containerToAttach, state)
		os.Exit(1)
	}
}
