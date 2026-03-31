package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/term"
)

// resolveVolume returns the Docker volume name for the given config.
// Returns empty for --once or encrypted modes (where volume name is derived
// from the password/user_id).
func resolveVolume(cfg config) string {
	if cfg.once {
		return ""
	}
	if cfg.ainstruct || cfg.encrypted {
		return ""
	}
	user, _ := os.UserHomeDir()
	user = filepath.Base(user)
	return "kilo-data-" + user
}

// isTerminal reports whether both stdin and stderr are connected to a TTY.
func isTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd()))
}

// dockerDaemonRunning returns true if `docker info` succeeds.
func dockerDaemonRunning() bool {
	_, err := exec.Command("docker", "info").CombinedOutput()
	return err == nil
}

// promptMissingTokens interactively prompts for MCP API tokens if not set.
func promptMissingTokens(volume string, encrypted bool, password string) {
	fmt.Fprintf(os.Stderr, "First-time setup: Enter API tokens for remote MCP servers.\n")
	fmt.Fprintf(os.Stderr, "Leave empty to skip a server.\n\n")

	if !isTerminal() {
		fmt.Fprintf(os.Stderr, "No TTY available. Tokens must be set manually in the volume.\n")
		return
	}

	token1 := promptToken("Context7 API token")
	token2 := promptToken("Ainstruct API token")

	if token1 != "" || token2 != "" {
		saveTokens(repoURL+":latest", volume, token1, token2, encrypted, password)
		if token1 != "" {
			os.Setenv("KD_CONTEXT7_TOKEN", token1)
		}
		if token2 != "" {
			os.Setenv("KD_AINSTRUCT_TOKEN", token2)
		}
	}
}

// printHelp displays usage, commands, options, and examples to stderr.
func printHelp() {
	help := `Usage: kilo-docker [--once] [--password] [--ainstruct] [--mcp] [--playwright] [--docker] [--ssh] [--zellij] [--network <name>] [command] [args...]

Commands:
  (no command)      Start Kilo in interactive mode
  run "prompt"      Run Kilo in autonomous mode with a prompt
  sessions [name]       List sessions or attach to one by name/index
  sessions cleanup [-y|-a] [name]  Remove a session (-y: skip confirm, -a: all exited)
  networks          List available Docker networks
  backup [-f]       Create backup of volume to tar.gz (auto-names with timestamp)
  restore <file> [-f] [--volume <name>]  Restore volume from backup
  init              Reset configuration (remove volume)
  cleanup           Remove volume, containers, images, and installed script
  install           Install kilo-docker as a global command (~/.local/bin)
  update            Pull the latest Docker image and update the installed script
  update-config     Download latest opencode.json template and merge with existing config
  help              Show this help message (wrapper)

Options:
  --once            Run a one-time session without persisting data (no volume)
  --password, -p    Protect volume with a password (encrypts tokens, derives volume name from password)
  --ainstruct       Encrypt tokens using Ainstruct user_id for volume naming
  --mcp             Enable MCP servers (prompts for Context7 and Ainstruct API tokens)
  --playwright      Start a Playwright MCP sidecar container for browser automation
  --docker          Mount Docker socket for container management from within Kilo
  --ssh             Enable SSH agent forwarding into the container
  --zellij          Start with Zellij multiplexer (detach: Ctrl+P Ctrl+Q, reattach: kilo-docker sessions)
  --network <name>  Connect the container to a Docker network
  --yes, -y         Auto-confirm all prompts (useful for piped/non-interactive installs)

Examples:
  kilo-docker                                    # interactive mode
  kilo-docker run "fix build errors"             # autonomous mode
  kilo-docker --password                         # interactive mode with encryption
  kilo-docker --ainstruct                        # interactive mode with Ainstruct encryption
  kilo-docker --once                             # one-time interactive session
  kilo-docker --mcp                              # with Context7 and Ainstruct MCP servers
  kilo-docker --playwright                       # with Playwright browser automation
  kilo-docker --docker                           # with Docker socket access
  kilo-docker --ssh                              # with SSH agent forwarding
  kilo-docker --zellij                           # start/reattach Zellij container
  kilo-docker sessions                           # list all sessions
  kilo-docker backup                             # create backup
  kilo-docker restore backup.tar.gz              # restore from backup
`
	fmt.Fprintf(os.Stderr, "%s", help)
}
