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
//	--password, -p    Encrypt tokens, derive volume name from password
//	--ainstruct       Authenticate with Ainstruct API for encryption
//	--mcp             Enable MCP servers (Context7, Ainstruct)
//	--playwright      Start Playwright MCP sidecar
//	--docker          Mount Docker socket
//	--ssh             Enable SSH agent forwarding
//	--zellij          Start with Zellij multiplexer
//	--network <name>  Connect to a Docker network
//	--yes, -y         Auto-confirm all prompts
//
// Commands:
//
//	(no command)      Start Kilo interactively
//	sessions          List/attach to sessions
//	backup            Create volume backup
//	restore           Restore from backup
//	install           Install as global command
//	update            Pull latest Docker image
//	init              Reset configuration
//	cleanup           Remove all artifacts
//	networks          List Docker networks
//	update-config     Merge config template
//	help              Show help
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func main() {
	cfg := parseFlags()
	autoConfirm = cfg.yes || !isTerminal()

	switch cfg.command {
	case "help":
		printHelp()
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
// Docker socket detection, volume resolution, Playwright sidecar, and finally
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

	// Docker socket
	dockerGID := ""
	if cfg.docker {
		if _, err := os.Stat("/var/run/docker.sock"); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: /var/run/docker.sock not found. Is Docker running?\n")
			os.Exit(1)
		}
		info, _ := os.Stat("/var/run/docker.sock")
		if info != nil {
			dockerGID = strconv.FormatUint(uint64(info.Sys().(*syscall.Stat_t).Gid), 10)
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
		ainstructSyncTokenExpiry = result.ExpiresIn
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

	// Build docker args
	dockerArgs := buildDockerArgs(cfg, dataVolume, pwd, containerName, containerState,
		sshAuthSock, dockerGID, kdContext7Token, kdAinstructToken,
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
		if err := execDockerAttach("attach", containerName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		resetTerminal()
	} else if containerState == "exited" || containerState == "created" {
		if err := execDockerAttach("start", "-ai", containerName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		resetTerminal()
	} else {
		runArgs := buildRunArgs(dockerArgs, image, cfg.args, isTerminal())
		execDocker(runArgs...)
	}
}

// buildRunArgs constructs the argument list for an interactive docker run
// command. The returned slice starts with "run", followed by the interactive
// flags (-it or -i), then the docker args, image, and any extra args.
func buildRunArgs(dockerArgs []string, image string, extraArgs []string, terminal bool) []string {
	args := []string{"run"}
	if terminal {
		args = append(args, "-it")
	} else {
		args = append(args, "-i")
	}
	args = append(args, dockerArgs...)
	args = append(args, image)
	args = append(args, extraArgs...)
	return args
}
