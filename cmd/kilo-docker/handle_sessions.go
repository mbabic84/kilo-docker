package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

type batchFlags struct {
	all         bool
	legacy      bool
	needsUpdate bool
}

// parseBatchFlags parses common batch operation flags and returns the parsed
// flags along with the remaining unparsed arguments.
func parseBatchFlags(args []string) (batchFlags, []string) {
	var bf batchFlags
	for len(args) > 0 {
		switch args[0] {
		case "-a", "--all":
			bf.all = true
		case "--legacy":
			bf.legacy = true
		case "--needs-update":
			bf.needsUpdate = true
		default:
			return bf, args
		}
		args = args[1:]
	}
	return bf, args
}

// handleSessions lists, attaches to, recreates, stops, or cleans up kilo-docker sessions.
func handleSessions(cfg config) {
	args := cfg.args

	// Hidden flag for shell tab-completion — must be checked before any
	// other arg parsing since it looks like a regular argument.
	if len(args) > 0 && args[0] == "--complete" {
		showSessionCompletions()
		return
	}

	cleanupMode := false
	var cleanupFlags batchFlags
	recreateMode := false
	stopMode := false
	var stopFlags batchFlags
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
		if len(args) > 0 && args[0] == "stop" {
			printCommandHelp("sessions stop")
			return
		}
		printCommandHelp("sessions")
		return
	}

	if len(args) > 0 && args[0] == "cleanup" {
		cleanupMode = true
		cleanupFlags, args = parseBatchFlags(args[1:])
	}

	if len(args) > 0 && args[0] == "recreate" {
		recreateMode = true
		args = args[1:]
	}

	if len(args) > 0 && args[0] == "stop" {
		stopMode = true
		stopFlags, args = parseBatchFlags(args[1:])
	}

	if len(args) > 0 {
		attachTarget = args[0]
	}

	sessions, err := getSessions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Auto-remove any exited --once sessions to prevent restarting one-time containers
	for _, s := range sessions {
		if dockerState(s.Name) != "running" && strings.Contains(s.Args, "--once") {
			_, _ = dockerRun("rm", "-f", s.Name)
		}
	}

	if stopMode {
		if stopFlags.all || stopFlags.legacy || stopFlags.needsUpdate {
			filtered := filterSessions(sessions, stopFlags.legacy, stopFlags.needsUpdate)
			if len(filtered) == 0 {
				fmt.Fprintf(os.Stderr, "No matching sessions to stop.\n")
				os.Exit(0)
			}
			for _, s := range filtered {
				state := dockerState(s.Name)
				if state != "running" {
					continue
				}
				if cfg.yes || promptConfirm("Stop session '"+s.Name+"'? [y/N]: ", cfg.yes) {
					utils.Log("[kilo-docker] Stopping session '%s'...\n", s.Name, utils.WithOutput())
					_, err = dockerRun("stop", s.Name)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error stopping session '%s': %v\n", s.Name, err)
						continue
					}
					fmt.Fprintf(os.Stderr, "Session '%s' stopped.\n", s.Name)
				} else {
					fmt.Fprintf(os.Stderr, "Skipped '%s'.\n", s.Name)
				}
			}
			return
		}

		if attachTarget == "" {
			fmt.Fprintf(os.Stderr, "Error: specify a session to stop (name or index), or use --all/--legacy/--needs-update\n")
			os.Exit(1)
		}
		containerName, err := resolveTargetWithSessions(attachTarget, sessions)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		state := dockerState(containerName)
		if state != "running" {
			fmt.Fprintf(os.Stderr, "Session '%s' is not running (current state: %s). Nothing to stop.\n", containerName, state)
			os.Exit(0)
		}

		utils.Log("[kilo-docker] Stopping session '%s'...\n", containerName, utils.WithOutput())
		_, err = dockerRun("stop", containerName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error stopping session '%s': %v\n", containerName, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Session '%s' stopped. Ports are now free.\n", containerName)
		fmt.Fprintf(os.Stderr, "To restart: kilo-docker sessions %s\n", containerName)
		return
	}

	if recreateMode {
		if attachTarget == "" {
			fmt.Fprintf(os.Stderr, "Error: specify a session to recreate (name or index)\n")
			os.Exit(1)
		}
		containerName, err := resolveTargetWithSessions(attachTarget, sessions)
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
		utils.Log("[kilo-docker] Parsed config: ssh=%v, yes=%v, services=%v\n", newCfg.ssh, newCfg.yes, newCfg.enabledServices)
		newCfg.command = "" // ensure runContainer creates a new container
		utils.Log("[kilo-docker] Recreate config after normalization: yes=%v, serialized=%q\n", newCfg.yes, serializeArgs(newCfg, newCfg.ssh))

		// Merge any flags the user passed on the command line as overrides
		newCfg = mergeOverrides(newCfg, cfg)

		// Only chdir to stored workspace if user didn't override --workspace
		if cfg.workspace == "" && targetSession.Workspace != "" {
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
		if cleanupFlags.all || cleanupFlags.legacy || cleanupFlags.needsUpdate {
			filtered := filterSessions(sessions, cleanupFlags.legacy, cleanupFlags.needsUpdate)
			if len(filtered) == 0 {
				fmt.Fprintf(os.Stderr, "No matching sessions to clean up.\n")
				os.Exit(0)
			}
			for _, s := range filtered {
				if cleanupFlags.all && !cleanupFlags.legacy && !cleanupFlags.needsUpdate && dockerState(s.Name) != "exited" {
					continue
				}
				if cfg.yes || promptConfirm("Remove session '"+s.Name+"'? [y/N]: ", cfg.yes) {
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
			containerToClean, err = resolveTargetWithSessions(attachTarget, sessions)
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
			containerToClean, err = resolveTargetWithSessions(selection, sessions)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}

		if cfg.yes || promptConfirm("Remove session '"+containerToClean+"'? [y/N]: ", cfg.yes) {
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

	containerToAttach, err := resolveTargetWithSessions(attachTarget, sessions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	_, username := resolveWorkspaceAndUsername()
	if containerUsesLegacyVolume(containerToAttach, deriveVolumeName(username)) {
		oldVolume := containerHomeVolume(containerToAttach)
		utils.Log("[kilo-docker] Session '%s' uses an outdated volume (%s).\n", containerToAttach, oldVolume, utils.WithOutput())
		utils.Log("[kilo-docker] Per-user volumes are now used for data isolation between users.\n", utils.WithOutput())
		if cfg.yes || promptConfirm("Recreate session with current per-user volume? [y/N]: ", cfg.yes) {
			var targetSession session
			for _, s := range sessions {
				if s.Name == containerToAttach {
					targetSession = s
					break
				}
			}

			if targetSession.Workspace != "" {
				if err := os.Chdir(targetSession.Workspace); err != nil {
					fmt.Fprintf(os.Stderr, "Error: cannot change to workspace '%s': %v\n", targetSession.Workspace, err)
					os.Exit(1)
				}
				utils.Log("[kilo-docker] Recreating session in workspace: %s\n", targetSession.Workspace, utils.WithOutput())
			}

			storedArgs := targetSession.Args
			parsedArgs := strings.Fields(storedArgs)
			newCfg := parseArgs(parsedArgs)
			newCfg.command = ""

			utils.Log("[kilo-docker] Removing legacy container '%s'...\n", containerToAttach, utils.WithOutput())
			_, _ = dockerRun("rm", "-f", containerToAttach)

			expectedVolume := deriveVolumeName(username)
			if oldVolume != "" && oldVolume != expectedVolume && !volumeExists(expectedVolume) {
				copyVolumeData(oldVolume, expectedVolume)
			}

			runContainer(newCfg)
			return
		}
	}

	state := dockerState(containerToAttach)
	switch state {
	case "running":
		_ = execDockerInteractive(containerToAttach, "kilo-entrypoint", "zellij-attach")
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
		// Check port conflicts before starting
		storedLabel := getContainerLabel(containerToAttach, "kilo.args")
		if storedLabel != "" {
			storedCfg := parseArgs(strings.Fields(storedLabel))
			if err := checkPortConflicts(storedCfg); err != nil {
				utils.LogError("[kilo-docker] %v\n", err, utils.WithOutput())
				os.Exit(1)
			}
		}
		utils.Log("[kilo-docker] Starting session '%s'...\n", containerToAttach, utils.WithOutput())
		if err := startAndWaitForRunning(containerToAttach); err != nil {
			utils.LogError("[kilo-docker] %v\n", err, utils.WithOutput())
			os.Exit(1)
		}
		_ = execDockerInteractive(containerToAttach, "kilo-entrypoint", "zellij-attach")
		handleSessionEnd(containerToAttach, false)
	default:
		fmt.Fprintf(os.Stderr, "Error: Container '%s' is in state '%s'.\n", containerToAttach, state)
		os.Exit(1)
	}
}
