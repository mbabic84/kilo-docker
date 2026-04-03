// kilo-docker is the host-side CLI for running Kilo in a Docker container.
//
// It replaces the original bash script (scripts/kilo-docker, ~1335 lines) with
// a Go binary that handles flag parsing, Docker orchestration, session management,
// backup/restore, and credential management. All container-side operations are
// delegated to the kilo-entrypoint binary via `docker run` subcommands.
//
// Usage:
//
//	kilo-docker [flags] [command] [args...]
//
// Flags:
//
//	--once            One-time session (no volume)
//	--password        Encrypt tokens, derive volume name from password
//	--port, -p        Map a port (host_port:container_port), repeatable
//	--ainstruct       Authenticate with Ainstruct API for encryption and sync
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

// runContainer orchestrates the full container launch: SSH setup, token loading,
// Service socket detection, volume resolution, Playwright sidecar, and finally
// exec'ing into the container.
func runContainer(cfg config) {
	if !dockerDaemonRunning() {
		fmt.Fprintf(os.Stderr, "Error: Docker daemon is not running.\n")
		os.Exit(1)
	}

	// SSH setup
	sshAuthSock := ""
	sshAgentStarted := false
	sshAgentPid := ""
	if cfg.ssh {
		sshAuthSock, _, sshAgentStarted = setupSSH()
		if sshAgentStarted {
			sshAgentPid = os.Getenv("SSH_AGENT_PID")
		}
	}

	// Collect host env vars needed by enabled services
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

	// Volume resolution
	dataVolume := resolveVolume(cfg)

	// Token loading
	kdContext7Token := os.Getenv("KD_CONTEXT7_TOKEN")
	kdAinstructToken := os.Getenv("KD_AINSTRUCT_TOKEN")
	var ainstructSyncToken, ainstructSyncRefreshToken string
	var ainstructSyncTokenExpiry int64

	if cfg.mcp && dataVolume != "" {
		var password string
		if cfg.encrypted || cfg.ainstruct {
			password = os.Getenv("VOLUME_PASSWORD")
		}
		token1, token2 := loadTokens(repoURL+":latest", dataVolume, cfg.encrypted || cfg.ainstruct, password)
		if token1 != "" {
			kdContext7Token = token1
		}
		if token2 != "" {
			kdAinstructToken = token2
		}

		if kdContext7Token == "" || kdAinstructToken == "" {
			promptMissingTokens(dataVolume, cfg.encrypted || cfg.ainstruct, password)
			// Refresh local vars from env (promptMissingTokens uses os.Setenv)
			if kdContext7Token == "" {
				kdContext7Token = os.Getenv("KD_CONTEXT7_TOKEN")
			}
			if kdAinstructToken == "" {
				kdAinstructToken = os.Getenv("KD_AINSTRUCT_TOKEN")
			}
		}
	}

	if cfg.ainstruct {
		result, err := ainstructLogin(repoURL + ":latest")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		dataVolume = deriveVolumeName(result.UserID)
		ainstructSyncToken = result.AccessToken
		ainstructSyncRefreshToken = result.RefreshToken
		ainstructSyncTokenExpiry = time.Now().Unix() + result.ExpiresIn
		os.Setenv("VOLUME_PASSWORD", result.UserID)

		// MCP token loading/prompting after volume is available
		if cfg.mcp {
			password := result.UserID
			token1, token2 := loadTokens(repoURL+":latest", dataVolume, true, password)
			if token1 != "" {
				kdContext7Token = token1
			}
			if token2 != "" {
				kdAinstructToken = token2
			}

			if kdContext7Token == "" || kdAinstructToken == "" {
				promptMissingTokens(dataVolume, true, password)
				// Refresh local vars from env (promptMissingTokens uses os.Setenv)
				if kdContext7Token == "" {
					kdContext7Token = os.Getenv("KD_CONTEXT7_TOKEN")
				}
				if kdAinstructToken == "" {
					kdAinstructToken = os.Getenv("KD_AINSTRUCT_TOKEN")
				}
			}
		}
	}

	// Playwright
	playwrightNetwork := cfg.network
	if cfg.playwright {
		if err := startPlaywright(&playwrightNetwork); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer cleanupPlaywright(playwrightNetwork)
	}

	// Network selection
	if cfg.networkFlag && cfg.network == "" && isTerminal() {
		if net, _ := selectNetwork(); net != "" {
			cfg.network = net
		}
	}
	if cfg.playwright {
		cfg.network = playwrightNetwork
	}

	// Container name
	pwd, _ := os.Getwd()
	containerName := deriveContainerName(pwd)

	// Container state
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

	// Workspace conflict check
	if !cfg.once {
		if strings.HasPrefix(pwd, "/home/kilo-t8x3m7kp") {
			fmt.Fprintf(os.Stderr, "Warning: Current directory (%s) overlaps with the container's home path.\n", pwd)
		}
	}

	// Build container args
	containerArgs := buildContainerArgs(cfg, dataVolume, pwd, containerName, containerState,
		sshAuthSock, hostEnvVars, kdContext7Token, kdAinstructToken,
		ainstructSyncToken, ainstructSyncRefreshToken, ainstructSyncTokenExpiry)

	// Clear sensitive data
	os.Unsetenv("VOLUME_PASSWORD")

	// Cleanup SSH agent
	if sshAgentStarted {
		defer cleanupSSH(sshAgentPid)
	}

	// Run
	image := repoURL + ":latest"
	if containerState == "running" {
		execDockerInteractive(containerName, "kilo-t8x3m7kp", "zellij", "attach", "--create", "kilo-docker")
		handleSessionEnd(containerName, cfg.once)
	} else if containerState == "exited" || containerState == "created" {
		dockerRun("start", "-d", containerName)
		time.Sleep(2 * time.Second)
		execDockerInteractive(containerName, "kilo-t8x3m7kp", "zellij", "attach", "--create", "kilo-docker")
		handleSessionEnd(containerName, cfg.once)
	} else {
		runArgs := buildRunArgs(containerArgs, image, cfg.args, false)
		runArgs[1] = "-d" // replace "-i" with "-d" for detached
		if _, err := dockerRunDetached(runArgs...); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		time.Sleep(2 * time.Second)
		execDockerInteractive(containerName, "kilo-t8x3m7kp", "zellij", "attach", "--create", "kilo-docker")
		handleSessionEnd(containerName, cfg.once)
	}
}

// buildRunArgs constructs the argument list for an interactive docker run
// command. The returned slice starts with "run", followed by the interactive
// flags (-it or -i), then the docker args, image, and any extra args.
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

// handleSessionEnd prints appropriate message after a session ends.
// With exec-based sessions, the container stays running via sleep infinity,
// so this always indicates the user detached from zellij.
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
