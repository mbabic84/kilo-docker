package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mbabic84/kilo-docker/pkg/services"
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
// If the user leaves both tokens empty, a skip marker is saved to the volume
// so the prompt won't appear again on subsequent runs.
func promptMissingTokens(volume string, encrypted bool, password string) {
	fmt.Fprintf(os.Stderr, "First-time setup: Enter API tokens for remote MCP servers.\n")
	fmt.Fprintf(os.Stderr, "Leave empty to skip (you won't be asked again).\n\n")

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
	} else {
		// User declined — save a skip marker so we don't prompt again
		if encrypted {
			saveSkipMarker(repoURL+":latest", volume, password)
		} else {
			savePlainSkipMarker(repoURL+":latest", volume)
		}
	}
}

// saveSkipMarker writes an encrypted marker file to the volume indicating
// the user has already been prompted and declined token entry. This prevents
// the prompt from appearing on every subsequent run.
func saveSkipMarker(image, volume, password string) {
	const kiloHome = "/home/kilo-t8x3m7kp"
	volumeMount := volume + ":" + kiloHome
	markerData := []byte("KD_TOKENS_SKIPPED=1\n")

	encData, err := encryptAES(markerData, password)
	if err != nil {
		return
	}
	uid := os.Getuid()
	gid := os.Getgid()
	encPath := kiloHome + "/.local/share/kilo/.tokens.skip"
	dockerRunWithStdin(string(encData),
		"-v", volumeMount,
		"--user", fmt.Sprintf("%d:%d", uid, gid),
		image,
		"sh", "-c", fmt.Sprintf(
			"mkdir -p \"$(dirname '%s')\" && cat > '%s' && chmod 600 '%s'",
			encPath, encPath, encPath),
	)
}

// savePlainSkipMarker writes a plain-text marker file to the volume indicating
// the user has already been prompted and declined token entry (non-encrypted mode).
func savePlainSkipMarker(image, volume string) {
	const kiloHome = "/home/kilo-t8x3m7kp"
	uid := os.Getuid()
	gid := os.Getgid()
	markerPath := kiloHome + "/.local/share/kilo/.tokens.skip"
	dockerRunWithStdin("KD_TOKENS_SKIPPED=1\n",
		"-v", volume+":"+kiloHome,
		"--user", fmt.Sprintf("%d:%d", uid, gid),
		image,
		"sh", "-c", fmt.Sprintf(
			"mkdir -p \"$(dirname '%s')\" && cat > '%s' && chmod 600 '%s'",
			markerPath, markerPath, markerPath),
	)
}

// printVersion prints kilo-docker and kilo versions.
func printVersion() {
	fmt.Printf("kilo-docker: %s\n", version)
	fmt.Printf("kilo: %s\n", kiloVersion)
}

// printHelp displays usage, commands, options, and examples to stderr.
func printHelp() {
	const w = 43 // column width for commands/options

	var svcLines []string
	for _, svc := range services.BuiltInServices {
		svcLines = append(svcLines, fmt.Sprintf("  %-*s %s", w-2, svc.Flag, svc.Description))
	}

	var cmdLines []string
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "sessions [name|index]", "List sessions or attach to one"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "sessions cleanup [-y]", "Remove a session (interactive selection)"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "sessions cleanup [-y] <name|index>", "Remove a session by name or index"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "sessions cleanup -y -a", "Remove all exited sessions"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "sessions recreate <name|index>", "Recreate a session (preserves volume, same flags)"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "networks", "List available Docker networks"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "backup [-f]", "Create backup of volume to tar.gz (auto-names with timestamp)"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "restore <file> [-f] [-v|--volume <name>]", "Restore volume from backup"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "init", "Reset configuration (remove volume)"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "cleanup", "Remove volume, containers, images, and installed binary"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "update", "Pull the latest Docker image and update the binary"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "update-config", "Download latest opencode.json template and merge with existing config"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "version", "Show kilo-docker and kilo versions"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "help", "Show this help message"))

	var optLines []string
	optLines = append(optLines, fmt.Sprintf("  %-*s %s", w-2, "--once", "Run a one-time session without persisting data (no volume)"))
	optLines = append(optLines, fmt.Sprintf("  %-*s %s", w-2, "--password", "Protect volume with a password (encrypts tokens, derives volume name from password)"))
	optLines = append(optLines, fmt.Sprintf("  %-*s %s", w-2, "--port, -p <host:container>", "Map a port (host_port:container_port). Can be specified multiple times"))
	optLines = append(optLines, fmt.Sprintf("  %-*s %s", w-2, "--ainstruct", "Authenticate with Ainstruct API, encrypt tokens, and enable file sync"))
	optLines = append(optLines, fmt.Sprintf("  %-*s %s", w-2, "--mcp", "Enable MCP servers (prompts for Context7 and Ainstruct API tokens)"))
	optLines = append(optLines, fmt.Sprintf("  %-*s %s", w-2, "--playwright", "Start a Playwright MCP sidecar container for browser automation"))
	optLines = append(optLines, fmt.Sprintf("  %-*s %s", w-2, "--ssh", "Enable SSH agent forwarding into the container"))
	optLines = append(optLines, fmt.Sprintf("  %-*s %s", w-2, "--network <name>", "Connect the container to a Docker network"))
	optLines = append(optLines, fmt.Sprintf("  %-*s %s", w-2, "--yes, -y", "Auto-confirm all prompts (useful for piped/non-interactive installs)"))

	var exLines []string
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker", "# start a shell in the container"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker --password", "# with encrypted tokens"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker --ainstruct", "# with Ainstruct authentication"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker --once", "# one-time session"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker --mcp", "# with Context7 and Ainstruct MCP servers"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker --playwright", "# with Playwright browser automation"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker --ssh", "# with SSH agent forwarding"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker -p 8080:8080 -p 3000:3000", "# with port mappings"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker --docker --zellij --go", "# with multiple services enabled"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker sessions", "# list all sessions"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker sessions recreate 1", "# recreate session with same flags"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker backup", "# create backup"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker restore backup.tar.gz", "# restore from backup"))

	help := strings.Join([]string{
		"Usage: kilo-docker [options] [services] [command] [args...]",
		"",
		"Commands:",
		strings.Join(cmdLines, "\n"),
		"",
		"Options:",
		strings.Join(optLines, "\n"),
		"",
		"Services:",
		strings.Join(svcLines, "\n"),
		"",
		"Examples:",
		strings.Join(exLines, "\n"),
		"",
	}, "\n")

	fmt.Fprintf(os.Stderr, "%s", help)
}
