package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mbabic84/kilo-docker/pkg/services"
	"github.com/mbabic84/kilo-docker/pkg/utils"
	"golang.org/x/term"
)

func resolveVolume(cfg config) string {
	if cfg.once {
		return ""
	}
	return "kilo-docker-data"
}

func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func dockerDaemonRunning() bool {
	_, err := exec.Command("docker", "info").CombinedOutput()
	return err == nil
}

func promptConfirm(message string, yes bool) bool {
	if yes {
		utils.Log("%sy\n", message)
		return true
	}
	fmt.Print(message)
	var response string
	fmt.Scanln(&response)
	return strings.ToLower(strings.TrimSpace(response)) == "y"
}

func printVersion() {
	fmt.Printf("kilo-docker: %s\n", version)
	fmt.Printf("kilo: %s\n", kiloVersion)
}

func printHelp() {
	const w = 43

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
	optLines = append(optLines, fmt.Sprintf("  %-*s %s", w-2, "--port, -p <host:container>", "Map a port (host_port:container_port). Can be specified multiple times"))
	optLines = append(optLines, fmt.Sprintf("  %-*s %s", w-2, "--mcp", "Enable remote MCP servers (Context7, Ainstruct)"))
	optLines = append(optLines, fmt.Sprintf("  %-*s %s", w-2, "--playwright", "Start a Playwright MCP sidecar container for browser automation"))
	optLines = append(optLines, fmt.Sprintf("  %-*s %s", w-2, "--ssh", "Enable SSH agent forwarding into the container"))
	optLines = append(optLines, fmt.Sprintf("  %-*s %s", w-2, "--network <name>", "Connect the container to a Docker network"))
	optLines = append(optLines, fmt.Sprintf("  %-*s %s", w-2, "--yes, -y", "Auto-confirm all prompts (useful for piped/non-interactive installs)"))

	var exLines []string
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker", "# start a shell in the container"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker --once", "# one-time session"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker --mcp", "# with MCP servers"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker --playwright", "# with Playwright browser automation"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker --ssh", "# with SSH agent forwarding"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker -p 8080:8080 -p 3000:3000", "# with port mappings"))
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