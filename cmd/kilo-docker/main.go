// kilo-docker is the host-side CLI for running Kilo in a Docker container.
//
// It replaces the original bash script (scripts/kilo-docker, ~1335 lines) with
// a Go binary that handles flag parsing, Docker orchestration, session management,
// and backup/restore. All container-side operations including authentication,
// crypto, and token management are delegated to the kilo-entrypoint binary.
//
// Usage:
//
//	kilo-docker [flags] [command] [args...]
//
// Flags:
//
//	--once            One-time session (no volume)
//	--port, -p        Map a port (host_port:container_port), repeatable
//	--volume, -v      Mount a volume (host_path:container_path), repeatable
//	--workspace, -w   Specify a custom workspace path (defaults to current directory)
//	--playwright      Start Playwright MCP sidecar
//	--ssh             Enable SSH agent forwarding
//	--network <name>  Connect to a Docker network
//	--yes, -y         Auto-confirm all prompts
//
// Commands:
//
//	sessions          List/attach to sessions
//	networks          List Docker networks
//	backup            Create volume backup
//	restore           Restore from backup
//	init              Reset configuration
//	cleanup           Remove all artifacts
//	update            Pull latest Docker image and update binary
//	update-config     Merge config template
//	version           Show versions
//	help              Show help
package main

import (
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

func main() {
	cfg := parseFlags()

	// Handle --help flag
	if cfg.help {
		if cfg.command != "" {
			if cfg.command == "sessions" && len(cfg.args) > 0 {
				printCommandHelp(cfg.command + " " + cfg.args[0])
			} else if cfg.command == "update" && len(cfg.args) > 0 {
				printCommandHelp(cfg.command + " " + cfg.args[0])
			} else {
				printCommandHelp(cfg.command)
			}
		} else {
			printHelp()
		}
		return
	}

	// Warn if running in non-interactive mode without -y flag
	if !isTerminal() && !cfg.yes {
		utils.LogWarn("[kilo-docker] Running in non-interactive mode (non-TTY). Use -y to auto-confirm prompts.\n")
	}

	switch cfg.command {
	case "help":
		if len(cfg.args) > 0 {
			if cfg.args[0] == "sessions" && len(cfg.args) > 1 {
				printCommandHelp(cfg.args[0] + " " + cfg.args[1])
			} else if cfg.args[0] == "update" && len(cfg.args) > 1 {
				printCommandHelp(cfg.args[0] + " " + cfg.args[1])
			} else {
				printCommandHelp(cfg.args[0])
			}
		} else {
			printHelp()
		}
	case "version":
		printVersion()
	case "networks":
		_ = listNetworks(cfg)
	case "sessions":
		handleSessions(cfg)
	case "update":
		handleUpdate(cfg)
	case "cleanup":
		handleCleanup(cfg)
	case "backup":
		handleBackup(cfg)
	case "restore":
		handleRestore(cfg)
	case "init":
		handleInit(cfg)
	case "playwright":
		handlePlaywright(cfg)
	default:
		runContainer(cfg)
	}
}

func runContainer(cfg config) {
	if !dockerDaemonRunning() {
		utils.LogError("[kilo-docker] Docker daemon is not running.\n")
		os.Exit(1)
	}

	// Ensure shared resources
	if err := EnsureSharedNetwork(); err != nil {
		utils.LogError("[kilo-docker] Failed to ensure shared network: %v\n", err)
		os.Exit(1)
	}

	if cfg.playwright {
		if err := EnsurePlaywrightVolume(); err != nil {
			utils.LogError("[kilo-docker] Failed to ensure Playwright volume: %v\n", err)
			os.Exit(1)
		}
	}

	pwd, _ := os.Getwd()
	workspace := pwd
	if cfg.workspace != "" {
		workspace = cfg.workspace
		if _, err := os.Stat(workspace); os.IsNotExist(err) {
			utils.LogError("[kilo-docker] Workspace path does not exist: %s\n", workspace)
			os.Exit(1)
		}
	}
	containerName := deriveContainerName(workspace)
	containerState := dockerState(containerName)
	if cfg.once {
		if containerState != "not_found" {
			_, _ = dockerRun("rm", "-f", containerName)
		}
		containerState = "not_found"
	} else {
		switch containerState {
		case "exited", "dead", "created":
			_, _ = dockerRun("rm", "-f", containerName)
			containerState = "not_found"
		}
	}

	if containerState == "running" {
		currentFlags := serializeForDisplay(cfg, cfg.ssh)
		storedFlags := getContainerLabel(containerName, "kilo.args")
		displayedStoredFlags := serializeStoredArgs(storedFlags)
		if !argsMatch(currentFlags, storedFlags) {
			utils.Log("[kilo-docker] Existing session uses different flags.\n", utils.WithOutput())
			utils.Log("[kilo-docker]   Existing: %s\n", displayedStoredFlags, utils.WithOutput())
			utils.Log("[kilo-docker]   Current:  %s\n", currentFlags, utils.WithOutput())
			if cfg.yes || promptConfirm("Recreate with new flags? [y/N]: ", cfg.yes) {
				_, _ = dockerRun("rm", "-f", containerName)
				containerState = "not_found"
			} else {
				_ = execDockerInteractive(containerName, "kilo-entrypoint", "zellij-attach")
				handleSessionEnd(containerName, cfg.once)
				return
			}
		}
	}

	sshAuthSock := ""
	sshAgentStarted := false
	sshAgentPid := ""
	if cfg.ssh {
		sshAuthSock, _, sshAgentStarted = setupSSH()
		if sshAgentStarted {
			sshAgentPid = os.Getenv("SSH_AGENT_PID")
		}
	}

	hostEnvVars := make(map[string]string)
	for _, svcName := range cfg.enabledServices {
		svc := getService(svcName)
		if svc == nil || svc.RequiresSocket == "" {
			continue
		}
		if _, err := os.Stat(svc.RequiresSocket); os.IsNotExist(err) {
			utils.LogError("%s not found. Is the host socket available?\n", svc.RequiresSocket)
			os.Exit(1)
		}
		info, _ := os.Stat(svc.RequiresSocket)
		if info != nil {
			gid := strconv.FormatUint(uint64(info.Sys().(*syscall.Stat_t).Gid), 10)
			for key := range svc.HostEnvVars {
				hostEnvVars[key] = gid
			}
		}
	}

	dataVolume := resolveVolume(cfg)

	if cfg.playwright {
		if err := startPlaywright(); err != nil {
			utils.LogError("%v\n", err)
			os.Exit(1)
		}
		defer cleanupPlaywright()
	}

	if cfg.networkFlag && len(cfg.networks) == 0 && isTerminal() {
		if net, _ := selectNetwork(); net != "" {
			cfg.networks = append(cfg.networks, net)
		}
	}

	if !cfg.once {
		if strings.HasPrefix(workspace, "/home/kd-") {
			utils.LogWarn("[kilo-docker] Current directory (%s) overlaps with the container's home path.\n", workspace)
		}
	}

	containerArgs := buildContainerArgs(cfg, dataVolume, workspace, containerName, containerState,
		sshAuthSock, hostEnvVars)

	if sshAgentStarted {
		defer cleanupSSH(sshAgentPid)
	}

	image := repoURL + ":latest"
	switch containerState {
	case "running":
		_ = execDockerInteractive(containerName, "kilo-entrypoint", "zellij-attach")
		handleSessionEnd(containerName, cfg.once)
	case "exited", "created":
		_, _ = dockerRun("start", "-d", containerName)
		_ = execDockerInteractive(containerName, "kilo-entrypoint", "zellij-attach")
		handleSessionEnd(containerName, cfg.once)
	default:
		startArgs := cfg.args
		runArgs := buildRunArgs(containerArgs, image, startArgs, false)
		runArgs[1] = "-d"
		utils.Log("[kilo-docker] Docker run args: docker %s\n", strings.Join(runArgs, " "))
		if _, err := dockerRunDetached(runArgs...); err != nil {
			utils.LogError("%v\n", err)
			os.Exit(1)
		}
		_ = execDockerInteractive(containerName, "kilo-entrypoint", "zellij-attach")
		handleSessionEnd(containerName, cfg.once)
	}
}

func buildRunArgs(containerArgs []string, image string, extraArgs []string, terminal bool) []string {
	args := []string{"run"}
	if terminal {
		args = append(args, "-it")
	} else {
		args = append(args, "-i")
	}
	args = append(args, containerArgs...)
	args = append(args, image)
	args = append(args, extraArgs...)
	return args
}

func handleSessionEnd(containerName string, onceMode bool) {
	resetTerminal()
	if onceMode {
		_, _ = dockerRun("rm", "-f", containerName)
		utils.Log("[kilo-docker] Session '%s' ended.\n", containerName, utils.WithOutput())
		utils.Log("[kilo-docker] Container removed (--once mode).\n", utils.WithOutput())
	} else {
		utils.Log("[kilo-docker] Detached from session '%s'.\n", containerName, utils.WithOutput())
		utils.Log("[kilo-docker] To re-attach, run: kilo-docker sessions %s\n", containerName, utils.WithOutput())
	}
}

func handlePlaywright(cfg config) {
	if cfg.help {
		printCommandHelp("playwright")
		return
	}
	if !dockerDaemonRunning() {
		utils.LogError("[kilo-docker] Docker daemon is not running.\n")
		os.Exit(1)
	}

	// Ensure shared network
	if err := EnsureSharedNetwork(); err != nil {
		utils.LogError("[kilo-docker] Failed to ensure shared network: %v\n", err)
		os.Exit(1)
	}

	// Handle volume recreation
	if cfg.playwrightRecreateVolume {
		if !cfg.yes {
			if !promptConfirm("This will delete all data in the Playwright volume. Continue? [y/N]: ", false) {
				utils.Log("[kilo-docker] Cancelled.\n", utils.WithOutput())
				return
			}
		}
		utils.Log("[kilo-docker] Removing browser automation data volume...\n", utils.WithOutput())
		_, _ = dockerRun("volume", "rm", PlaywrightVolumeName)
	}

	// Ensure volume (creates if removed, or validates existing)
	if err := EnsurePlaywrightVolume(); err != nil {
		utils.LogError("[kilo-docker] Failed to ensure Playwright volume: %v\n", err)
		os.Exit(1)
	}

	// Remove existing container if any
	utils.Log("[kilo-docker] Preparing browser automation sidecar...\n", utils.WithOutput())
	_, _ = dockerRun("rm", "-f", SharedPlaywrightContainerName)

	// Pull latest image
	utils.Log("[kilo-docker] Starting browser automation sidecar...\n", utils.WithOutput())
	if _, err := dockerRun("pull", "mcr.microsoft.com/playwright/mcp"); err != nil {
		utils.LogError("[playwright] Failed to pull image: %v\n", err)
		os.Exit(1)
	}

	// Start the container with correct uid/gid
	if err := startPlaywright(); err != nil {
		utils.LogError("[playwright] Failed to start: %v\n", err)
		os.Exit(1)
	}

	utils.Log("[kilo-docker] Browser automation sidecar ready.\n", utils.WithOutput())
}
