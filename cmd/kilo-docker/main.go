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
//	--mcp             Enable MCP servers (Context7, Ainstruct)
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
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	cfg := parseFlags()
	autoConfirm = cfg.yes || !isTerminal()

	switch cfg.command {
	case "help":
		printHelp()
	case "version":
		printVersion()
	case "networks":
		listNetworks()
	case "sessions":
		handleSessions(cfg)
	case "update":
		handleUpdate()
	case "cleanup":
		handleCleanup()
	case "backup":
		handleBackup(cfg)
	case "restore":
		handleRestore(cfg)
	case "init":
		handleInit(cfg)
	case "update-config":
		handleUpdateConfig(cfg)
	default:
		runContainer(cfg)
	}
}

func runContainer(cfg config) {
	if !dockerDaemonRunning() {
		fmt.Fprintf(os.Stderr, "Error: Docker daemon is not running.\n")
		os.Exit(1)
	}

	pwd, _ := os.Getwd()
	containerName := deriveContainerName(pwd)
	containerState := dockerState(containerName)
	if cfg.once {
		if containerState != "not_found" {
			dockerRun("rm", "-f", containerName)
		}
		containerState = "not_found"
	} else {
		switch containerState {
		case "exited", "dead", "created":
			dockerRun("rm", "-f", containerName)
			containerState = "not_found"
		}
	}

	if containerState == "running" {
		currentFlags := serializeArgs(cfg, cfg.ssh)
		storedFlags := getContainerLabel(containerName, "kilo.args")
		if currentFlags != storedFlags {
			fmt.Fprintf(os.Stderr, "Existing session uses different flags.\n")
			fmt.Fprintf(os.Stderr, "  Existing: %s\n", storedFlags)
			fmt.Fprintf(os.Stderr, "  Current:  %s\n", currentFlags)
			if cfg.yes || confirmPrompt("Recreate with new flags? [y/N]: ") {
				dockerRun("rm", "-f", containerName)
				containerState = "not_found"
			} else {
				execDockerInteractive(containerName, "kilo-entrypoint", "zellij-attach")
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
			fmt.Fprintf(os.Stderr, "Error: %s not found. Is the host socket available?\n", svc.RequiresSocket)
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

	playwrightNetwork := cfg.network
	if cfg.playwright {
		if err := startPlaywright(&playwrightNetwork); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer cleanupPlaywright(playwrightNetwork)
	}

	if cfg.networkFlag && cfg.network == "" && isTerminal() {
		if net, _ := selectNetwork(); net != "" {
			cfg.network = net
		}
	}
	if cfg.playwright {
		cfg.network = playwrightNetwork
	}

	if !cfg.once {
		if strings.HasPrefix(pwd, "/home/kd-") {
			fmt.Fprintf(os.Stderr, "Warning: Current directory (%s) overlaps with the container's home path.\n", pwd)
		}
	}

	containerArgs := buildContainerArgs(cfg, dataVolume, pwd, containerName, containerState,
		sshAuthSock, hostEnvVars)

	if sshAgentStarted {
		defer cleanupSSH(sshAgentPid)
	}

	image := repoURL + ":latest"
	if containerState == "running" {
		execDockerInteractive(containerName, "kilo-entrypoint", "zellij-attach")
		handleSessionEnd(containerName, cfg.once)
	} else if containerState == "exited" || containerState == "created" {
		dockerRun("start", "-d", containerName)
		time.Sleep(2 * time.Second)
		execDockerInteractive(containerName, "kilo-entrypoint", "zellij-attach")
		handleSessionEnd(containerName, cfg.once)
	} else {
		runArgs := buildRunArgs(containerArgs, image, cfg.args, false)
		runArgs[1] = "-d"
		if _, err := dockerRunDetached(runArgs...); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		time.Sleep(2 * time.Second)
		execDockerInteractive(containerName, "kilo-entrypoint", "zellij-attach")
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
		dockerRun("rm", "-f", containerName)
		fmt.Fprintf(os.Stderr, "\nSession '%s' ended.\n", containerName)
		fmt.Fprintf(os.Stderr, "Container removed (--once mode).\n")
	} else {
		fmt.Fprintf(os.Stderr, "\nDetached from session '%s'.\n", containerName)
		fmt.Fprintf(os.Stderr, "To re-attach, run: kilo-docker sessions %s\n", containerName)
	}
}

func confirmPrompt(message string) bool {
	if autoConfirm {
		fmt.Fprintf(os.Stderr, "%sy\n", message)
		return true
	}
	fmt.Print(message)
	var response string
	fmt.Scanln(&response)
	return strings.ToLower(strings.TrimSpace(response)) == "y"
}