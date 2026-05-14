package main

import (
	"fmt"
	"math"
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
	fd, ok := fileDescriptor(os.Stdout.Fd())
	return ok && term.IsTerminal(fd)
}

func dockerDaemonRunning() bool {
	_, err := exec.Command("docker", "info").CombinedOutput()
	return err == nil
}

func promptConfirm(message string, yes bool) bool {
	if yes {
		utils.Log("[kilo-docker] %sy\n", message, utils.WithOutput())
		return true
	}
	fmt.Print(message)
	var response string
	_, _ = fmt.Scanln(&response)
	return strings.ToLower(strings.TrimSpace(response)) == "y"
}

func fileDescriptor(fd uintptr) (int, bool) {
	if fd > uintptr(math.MaxInt) {
		return 0, false
	}
	return int(fd), true
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
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "sessions", "Manage sessions (use sessions -h for subcommands)"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "networks", "List available Docker networks"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "playwright", "Recreate Playwright MCP container"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "backup", "Create backup of volume"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "restore", "Restore volume from backup"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "init", "Reset configuration"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "cleanup", "Remove all artifacts"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "update", "Update binary and/or config (use update -h for subcommands)"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "version", "Show versions"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "help", "Show this help message"))

	var optLines []string
	optLines = append(optLines, formatFlagHelp())

	var exLines []string
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker", "# start a shell in the container"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker --once", "# one-time session"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker --playwright", "# with Playwright browser automation"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker --ssh", "# with SSH agent forwarding"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker -p 8080:8080 -p 3000:3000", "# with port mappings"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker sessions", "# list all sessions"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker sessions -h", "# see sessions help"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker backup", "# create backup"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-docker restore -h", "# see restore help"))

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
		"Use 'kilo-docker <command> -h' for more details about a command.",
	}, "\n")

	fmt.Fprintf(os.Stderr, "%s", help)
}

func printCommandHelp(command string) {
	var help string
	switch command {
	case "sessions":
		help = `Usage: kilo-docker sessions [command] [options]

List sessions, attach to one, or stop a running session.

Commands:
  (no command)          List all sessions and attach interactively
  cleanup               Remove sessions
  recreate              Recreate a session with the same flags
  stop                  Stop a running session (frees ports, preserves container)

Options:
  -h, --help            Show this help message

Examples:
  kilo-docker sessions                          # list sessions
  kilo-docker sessions -h                       # show this help
`
	case "sessions cleanup":
		help = `Usage: kilo-docker sessions cleanup [options] [name|index]

Remove one or more sessions.

Options:
  -a, --all             Remove all exited sessions
  -y, --yes             Skip confirmation prompts
  -h, --help            Show this help message

Without flags, shows interactive selection.
With -a, prompts for each session (or skips if -y is set).

Examples:
  kilo-docker sessions cleanup                  # interactive selection
  kilo-docker sessions cleanup 1                # remove session 1
  kilo-docker sessions cleanup -a              # remove all exited (with prompt)
  kilo-docker sessions cleanup -a -y            # remove all exited (no prompt)
  kilo-docker sessions cleanup -h              # show this help
`
	case "sessions recreate":
		help = `Usage: kilo-docker sessions recreate <name|index>

Recreate a session with the same configuration.

This removes the old container but preserves the volume,
then starts a fresh container with the same flags.

Examples:
  kilo-docker sessions recreate 1               # recreate session 1
  kilo-docker sessions recreate my-session     # recreate by name
  kilo-docker sessions recreate -h             # show this help
`
	case "sessions stop":
		help = `Usage: kilo-docker sessions stop <name|index>

Stop a running session, freeing its ports while preserving the
container and volume for later restart.

Use 'kilo-docker sessions <name>' to restart the session.

Examples:
  kilo-docker sessions stop 1                   # stop session 1
  kilo-docker sessions stop my-session          # stop by name
  kilo-docker sessions stop -h                  # show this help
`
	case "networks":
		help = `Usage: kilo-docker networks

List available Docker networks.

Shows all Docker networks on the host, including the
kilo-shared network used by kilo-docker services.

Examples:
  kilo-docker networks
  kilo-docker networks -h
`
	case "playwright":
		help = `Usage: kilo-docker playwright [options]

Recreate the Playwright MCP container with the latest image.

Options:
  -v, --volume          Recreate the Playwright volume (deletes all data)
  -y, --yes             Auto-confirm prompts
  -h, --help            Show this help message

This will:
  - Ensure the shared network (kilo-shared) exists
  - Ensure/validate the shared volume (kilo-playwright-output) exists
  - Pull the latest mcr.microsoft.com/playwright/mcp image
  - Start a new container with correct UID/GID

The container runs on the kilo-shared network and uses the
kilo-playwright-output volume for storing screenshots and output.

Examples:
  kilo-docker playwright                # recreate container
  kilo-docker playwright -v             # recreate container and volume
  kilo-docker playwright -h            # show this help
`
	case "backup":
		help = `Usage: kilo-docker backup [options]

Create a backup of the kilo-docker volume.

Options:
  -f, --force             Overwrite existing backup file
  -h, --help              Show this help message

Creates a tar.gz archive of the volume data. By default,
auto-generates a timestamped filename.

Examples:
  kilo-docker backup                     # backup with auto-generated name
  kilo-docker backup -f my-backup.tar.gz # backup to specific file
  kilo-docker backup -h                  # show this help
`
	case "restore":
		help = `Usage: kilo-docker restore <file> [options]

Restore kilo-docker volume from a backup.

Arguments:
  <file>                 Path to the backup tar.gz file

Options:
  -f, --force            Overwrite existing data
  -v, --volume <name>    Restore to a specific volume (default: kilo-docker-data)
  -h, --help            Show this help message

Examples:
  kilo-docker restore backup.tar.gz
  kilo-docker restore backup.tar.gz -f
  kilo-docker restore backup.tar.gz -v my-volume
  kilo-docker restore -h
`
	case "init":
		help = `Usage: kilo-docker init [options]

Reset configuration by removing the volume.

WARNING: This deletes all data in the kilo-docker volume.

Options:
  -y, --yes             Skip confirmation
  -h, --help            Show this help message

Examples:
  kilo-docker init
  kilo-docker init -y
  kilo-docker init -h
`
	case "cleanup":
		help = `Usage: kilo-docker cleanup [options]

Remove all kilo-docker artifacts from the system.

WARNING: This removes:
  - The kilo-docker-data volume
  - All kilo-docker containers
  - The kilo-docker image
  - The kilo-docker binary

Options:
  -y, --yes             Skip confirmation
  -h, --help            Show this help message

Examples:
  kilo-docker cleanup
  kilo-docker cleanup -y
  kilo-docker cleanup -h
`
	case "update":
		help = `Usage: kilo-docker update [command]

Update kilo-docker binary and/or merge configuration.

Commands:
  (no command)          Pull latest Docker image and update binary
  config                Download latest opencode.json and merge with existing

Options:
  -h, --help            Show this help message

Examples:
  kilo-docker update                    # update binary
  kilo-docker update -h                 # show this help
  kilo-docker update config             # merge config
  kilo-docker update config -h          # see config options
`
	case "update config":
		help = `Usage: kilo-docker update config

Download the latest opencode.json template and merge with existing config.

This will pull the latest configuration template from the repository
and merge it with your existing config in the volume.

Options:
  -h, --help            Show this help message

Examples:
  kilo-docker update config
  kilo-docker update config -h
`
	case "version":
		help = `Usage: kilo-docker version

Show kilo-docker and kilo versions.

Examples:
  kilo-docker version
  kilo-docker version -h
`
	case "help":
		help = `Usage: kilo-docker help [command]

Show help for kilo-docker commands.

Arguments:
  [command]             Optional command to get help for

Examples:
  kilo-docker help
  kilo-docker help sessions
  kilo-docker help sessions cleanup
`
	default:
		help = fmt.Sprintf("Unknown command: %s\nRun 'kilo-docker help' for usage.\n", command)
	}
	fmt.Fprintf(os.Stderr, "%s", help)
}
