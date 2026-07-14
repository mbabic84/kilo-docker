// kilo-entrypoint is the container entrypoint binary for the kilo-docker image.
//
// When invoked with no arguments (or as the default ENTRYPOINT), it performs
// container initialization: user/group setup, tool downloads, SSH known_hosts,
// config directory creation, privilege drop, and MCP server toggling.
//
// When invoked with a subcommand, it delegates to the appropriate handler:
//
//	update-config   Download config template, merge with existing config
//	backup          Create tar.gz of KILO_HOME
//	restore         Extract tar.gz into KILO_HOME with ownership fix
//	mcp-config      Apply MCP enabled states from KD_MCP_* env vars
//	mcp-tokens      Interactive token management
//	sync            Start ainstruct file watcher + REST sync
//	resync          Delete all remote documents and re-push local files
package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

// subcommands lists the internal subcommands handled by kilo-entrypoint.
// Any argument NOT in this map is passed through to exec.LookPath for
// direct binary execution (e.g. "kilo", "sh", "bash").
var subcommands = map[string]bool{
	"update-config":   true,
	"backup":          true,
	"restore":         true,
	"mcp-config":      true,
	"mcp-tokens":      true,
	"sync":            true,
	"resync":          true,
	"zellij-attach":   true,
	"print-env":       true,
	"custom-envs":     true,
	"completions":     true,
	"help":            true,
}

// entrypointArgs captures the result of parsing kilo-entrypoint command-line
// arguments, mirroring the custom parser used by kilo-docker.
type entrypointArgs struct {
	help    bool
	command string
	args    []string
}

// parseEntrypointArgs parses raw command-line arguments. It detects -h/--help,
// treats the first non-flag argument as the command, and collects the rest as
// command arguments. This mirrors kilo-docker's custom flag parsing.
func parseEntrypointArgs(rawArgs []string) entrypointArgs {
	var parsed entrypointArgs
	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		if arg == "-h" || arg == "--help" {
			parsed.help = true
			continue
		}
		if parsed.command == "" && strings.HasPrefix(arg, "-") {
			// Skip unknown leading flags, matching Go's flag.Parse behavior.
			continue
		}
		if parsed.command == "" {
			parsed.command = arg
		} else {
			parsed.args = append(parsed.args, arg)
		}
	}
	return parsed
}

// resolveCommand checks if name is a known internal subcommand.
// If not, it resolves the name to an executable binary via LookPath.
// Returns (binaryPath, true) for pass-through commands, ("", false) for
// internal subcommands.
func resolveCommand(name string) (string, bool) {
	if subcommands[name] {
		return "", false
	}
	binary, err := exec.LookPath(name)
	if err != nil {
		return "", true // unknown, but still a pass-through (will error in main)
	}
	return binary, true
}

func printHelp() {
	const w = 40

	var cmdLines []string
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "help", "Show this help message"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "update-config", "Download config template, merge with existing config"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "backup [path]", "Create tar.gz of KILO_HOME (default: /tmp/backup.tar.gz)"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "restore [path]", "Extract tar.gz into KILO_HOME with ownership fix"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "mcp-config", "Apply MCP enabled states from encrypted token storage"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "mcp-tokens", "Interactive token management"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "sync", "Start ainstruct file watcher + REST sync"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "sync ls", "List all ainstruct sync files"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "sync rm <file>", "Remove a specific sync file (local and remote)"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "resync", "Delete all remote documents and re-push local files"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "zellij-attach", "Attach to existing zellij session"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "print-env", "Print export statements for current tokens and custom envs"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "custom-envs", "Manage user-defined custom environment variables"))
	cmdLines = append(cmdLines, fmt.Sprintf("  %-*s %s", w-2, "completions <shell>", "Generate shell completion scripts"))

	var exLines []string
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-entrypoint zellij-attach", "# attach to existing zellij session"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-entrypoint sync", "# start ainstruct file watcher"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-entrypoint backup", "# create backup in /tmp/backup.tar.gz"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-entrypoint help backup", "# see backup help"))
	exLines = append(exLines, fmt.Sprintf("  %-*s %s", w-2, "kilo-entrypoint sync -h", "# see sync help"))

	help := strings.Join([]string{
		"kilo-entrypoint - Container entrypoint for kilo-docker",
		"",
		"Usage: kilo-entrypoint [subcommand]",
		"",
		"With no arguments, runs container initialization.",
		"",
		"Subcommands:",
		strings.Join(cmdLines, "\n"),
		"",
		"Custom Envs Subcommands:",
		fmt.Sprintf("  %-*s %s", w-2, "custom-envs list", "List all custom envs (keys + masked values)"),
		fmt.Sprintf("  %-*s %s", w-2, "custom-envs get <key>", "Print raw value of a custom env to stdout"),
		fmt.Sprintf("  %-*s %s", w-2, "custom-envs add <key> <value>", "Add a new custom env"),
		fmt.Sprintf("  %-*s %s", w-2, "custom-envs edit <key> <value>", "Edit an existing custom env"),
		fmt.Sprintf("  %-*s %s", w-2, "custom-envs remove <key>", "Remove a custom env"),
		"",
		"Examples:",
		strings.Join(exLines, "\n"),
		"",
		"Any other argument is passed through to exec.LookPath for",
		"direct binary execution (e.g. \"kilo\", \"sh\", \"bash\").",
		"",
		"Use 'kilo-entrypoint <command> -h' for more details about a command.",
	}, "\n")

	fmt.Fprintf(os.Stderr, "%s\n", help)
}

func printCommandHelp(command string) {
	var help string
	switch command {
	case "update-config":
		help = `Usage: kilo-entrypoint update-config

Download the latest config template and merge it with the existing
config in KILO_HOME.

Options:
  -h, --help            Show this help message

Examples:
  kilo-entrypoint update-config
  kilo-entrypoint update-config -h
`
	case "backup":
		help = `Usage: kilo-entrypoint backup [path]

Create a tar.gz archive of KILO_HOME.

Arguments:
  [path]                Destination archive path (default: /tmp/backup.tar.gz)

Options:
  -h, --help            Show this help message

Examples:
  kilo-entrypoint backup                    # create /tmp/backup.tar.gz
  kilo-entrypoint backup /tmp/my-backup.tar.gz
  kilo-entrypoint backup -h                 # show this help
`
	case "restore":
		help = `Usage: kilo-entrypoint restore [path]

Extract a tar.gz archive into KILO_HOME with ownership fix.

Arguments:
  [path]                Source archive path (default: /tmp/backup.tar.gz)

Options:
  -h, --help            Show this help message

Examples:
  kilo-entrypoint restore                   # restore /tmp/backup.tar.gz
  kilo-entrypoint restore /tmp/my-backup.tar.gz
  kilo-entrypoint restore -h                # show this help
`
	case "mcp-config":
		help = `Usage: kilo-entrypoint mcp-config

Apply MCP enabled states from encrypted token storage.

This reads the encrypted tokens and KD_MCP_* environment variables,
then updates the local MCP configuration accordingly.

Options:
  -h, --help            Show this help message

Examples:
  kilo-entrypoint mcp-config
  kilo-entrypoint mcp-config -h
`
	case "mcp-tokens":
		help = `Usage: kilo-entrypoint mcp-tokens

Interactive token management for MCP services.

Prompts for and stores Context7, Ainstruct, and other MCP tokens
in an encrypted file under KILO_HOME.

Options:
  -h, --help            Show this help message

Examples:
  kilo-entrypoint mcp-tokens
  kilo-entrypoint mcp-tokens -h
`
	case "sync":
		help = `Usage: kilo-entrypoint sync [command]

Start ainstruct file watcher and REST sync, or manage sync files.

Commands:
  (no command)          Start the file watcher and sync loop
  ls                    List all ainstruct sync files
  rm <file>             Remove a specific sync file (local and remote)

Options:
  -h, --help            Show this help message

Use 'kilo-entrypoint sync ls -h' or 'kilo-entrypoint sync rm -h' for
subcommand help.

Examples:
  kilo-entrypoint sync                      # start watcher
  kilo-entrypoint sync ls                   # list sync files
  kilo-entrypoint sync -h                   # show this help
`
	case "sync ls":
		help = `Usage: kilo-entrypoint sync ls

List all ainstruct sync files.

Options:
  -h, --help            Show this help message

Examples:
  kilo-entrypoint sync ls
  kilo-entrypoint sync ls -h
`
	case "sync rm":
		help = `Usage: kilo-entrypoint sync rm <file>

Remove a specific sync file, both locally and remotely.

Arguments:
  <file>                Path of the sync file to remove

Options:
  -h, --help            Show this help message

Examples:
  kilo-entrypoint sync rm ./docs/readme.md
  kilo-entrypoint sync rm -h                # show this help
`
	case "resync":
		help = `Usage: kilo-entrypoint resync

Delete all remote documents and re-push every local sync file.

Use this when the remote state is out of sync with the local files.

Options:
  -h, --help            Show this help message

Examples:
  kilo-entrypoint resync
  kilo-entrypoint resync -h
`
	case "zellij-attach":
		help = `Usage: kilo-entrypoint zellij-attach

Attach to the existing zellij session inside the container.

This is the normal entrypoint used by kilo-docker when reconnecting
to a running container.

Options:
  -h, --help            Show this help message

Examples:
  kilo-entrypoint zellij-attach
  kilo-entrypoint zellij-attach -h
`
	case "print-env":
		help = `Usage: kilo-entrypoint print-env

Print export statements for the currently stored tokens and custom envs.

Output is suitable for eval-ing in a shell.

Options:
  -h, --help            Show this help message

Examples:
  kilo-entrypoint print-env
  kilo-entrypoint print-env -h
`
	case "custom-envs":
		help = `Usage: kilo-entrypoint custom-envs <command>

Manage user-defined custom environment variables.

Custom envs are stored encrypted and can be printed via print-env.

Commands:
  list                  List all custom envs (keys + masked values)
  get <key>             Print raw value of a custom env to stdout
  add <key> <value>     Add a new custom env
  edit <key> <value>    Edit an existing custom env
  remove <key>          Remove a custom env

Options:
  -h, --help            Show this help message

Examples:
  kilo-entrypoint custom-envs list
  kilo-entrypoint custom-envs get MY_VAR
  kilo-entrypoint custom-envs -h
`
	case "custom-envs list":
		help = `Usage: kilo-entrypoint custom-envs list

List all custom envs with masked values.

Options:
  -h, --help            Show this help message

Examples:
  kilo-entrypoint custom-envs list
  kilo-entrypoint custom-envs list -h
`
	case "custom-envs get":
		help = `Usage: kilo-entrypoint custom-envs get <key>

Print the raw value of a custom env to stdout.

Arguments:
  <key>                 Name of the custom env

Options:
  -h, --help            Show this help message

Examples:
  kilo-entrypoint custom-envs get MY_VAR
  kilo-entrypoint custom-envs get -h
`
	case "custom-envs add":
		help = `Usage: kilo-entrypoint custom-envs add <key> <value>

Add a new custom env.

Arguments:
  <key>                 Name of the custom env
  <value>               Value of the custom env

Options:
  -h, --help            Show this help message

Examples:
  kilo-entrypoint custom-envs add MY_VAR my-value
  kilo-entrypoint custom-envs add -h
`
	case "custom-envs edit":
		help = `Usage: kilo-entrypoint custom-envs edit <key> <value>

Edit an existing custom env.

Arguments:
  <key>                 Name of the custom env
  <value>               New value of the custom env

Options:
  -h, --help            Show this help message

Examples:
  kilo-entrypoint custom-envs edit MY_VAR new-value
  kilo-entrypoint custom-envs edit -h
`
	case "custom-envs remove":
		help = `Usage: kilo-entrypoint custom-envs remove <key>

Remove a custom env.

Arguments:
  <key>                 Name of the custom env

Options:
  -h, --help            Show this help message

Examples:
  kilo-entrypoint custom-envs remove MY_VAR
  kilo-entrypoint custom-envs remove -h
`
	case "completions":
		help = `Usage: kilo-entrypoint completions <shell>

Generate shell completion scripts for kilo-entrypoint.

Arguments:
  <shell>             Target shell: bash, zsh, or fish

Examples:
  kilo-entrypoint completions bash
  kilo-entrypoint completions zsh
  kilo-entrypoint completions fish
`
	case "help":
		help = `Usage: kilo-entrypoint help [command]

Show help for kilo-entrypoint commands.

Arguments:
  [command]             Optional command to get help for

Examples:
  kilo-entrypoint help
  kilo-entrypoint help backup
  kilo-entrypoint help sync ls
`
	default:
		help = fmt.Sprintf("Unknown command: %s\nRun 'kilo-entrypoint help' for usage.\n", command)
	}
	fmt.Fprintf(os.Stderr, "%s\n", help)
}

func runPrintEnv() {
	homeDir, _, _, userID := loadUserConfig()
	if homeDir == "" || userID == "" {
		return
	}

	context7, ainstruct, _, _, _, patExpiry, err := loadEncryptedTokens(homeDir, userID)
	if err != nil {
		return
	}

	fmt.Printf("export KD_MCP_CONTEXT7_TOKEN=%q\n", context7)
	fmt.Printf("export KD_MCP_AINSTRUCT_TOKEN=%q\n", ainstruct)
	if patExpiry != "" {
		fmt.Printf("export KD_AINSTRUCT_PAT_EXPIRY=%q\n", patExpiry)
	} else {
		fmt.Printf("# KD_AINSTRUCT_PAT_EXPIRY not set\n")
	}

	customEnvs, err := loadCustomEnvs(homeDir, userID)
	if err != nil {
		if !os.IsNotExist(err) {
			utils.Log("[kilo-docker] Failed to load custom envs: %v\n", err)
		}
		return
	}

	if len(customEnvs) == 0 {
		return
	}

	utils.Log("[kilo-docker] Loading %d custom envs\n", len(customEnvs))

	keys := make([]string, 0, len(customEnvs))
	for k := range customEnvs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("export %s=%q\n", k, customEnvs[k])
	}
}

func main() {
	parsed := parseEntrypointArgs(os.Args[1:])

	// Set HOME to the user's home directory before any log calls so that
	// getLogFile() resolves the log path under the correct user home
	// instead of /root (the container's default HOME).
	if homeDir, _, _, _ := loadUserConfig(); homeDir != "" {
		_ = os.Setenv("HOME", homeDir)
	}

	utils.Log("[main] Entrypoint started, os.Args=%v\n", os.Args)

	// Handle -h/--help flag, mirroring kilo-docker.
	if parsed.help {
		if parsed.command != "" {
			printCommandHelp(commandWithSubcommand(parsed.command, parsed.args))
		} else {
			printHelp()
		}
		return
	}

	// Handle help [command] syntax, mirroring kilo-docker.
	if parsed.command == "help" {
		if len(parsed.args) > 0 {
			printCommandHelp(commandWithSubcommand(parsed.args[0], parsed.args[1:]))
		} else {
			printHelp()
		}
		return
	}

	hasSubcommand := parsed.command != ""
	alreadyInitialized := func() bool {
		_, err := os.Stat("/tmp/.kilo-initialized")
		return err == nil
	}

	if !hasSubcommand {
		if alreadyInitialized() {
			utils.Log("[main] Container already initialized, sleeping for exec\n")
			if err := syscall.Exec("/bin/sleep", []string{"sleep", "infinity"}, os.Environ()); err != nil {
				utils.LogError("[main] sleep exec failed: %v\n", err, utils.WithOutput())
				os.Exit(1)
			}
			return
		}
		if err := runInit(); err != nil {
			utils.LogError("[main] init error: %v\n", err, utils.WithOutput())
			os.Exit(1)
		}
		return
	}

	name := parsed.command
	binary, passthrough := resolveCommand(name)
	if passthrough {
		if binary == "" {
			utils.LogError("[main] unknown subcommand or command: %s\n", name, utils.WithOutput())
			os.Exit(1)
		}
		remaining := append([]string{name}, parsed.args...)
		if err := syscall.Exec(binary, remaining, os.Environ()); err != nil {
			utils.LogError("[main] exec %s: %v\n", name, err, utils.WithOutput())
			os.Exit(1)
		}
		return
	}

	switch name {
	case "update-config":
		if err := runUpdateConfig(); err != nil {
			utils.LogError("[main] update-config error: %v\n", err, utils.WithOutput())
			os.Exit(1)
		}
	case "backup":
		outputPath := "/tmp/backup.tar.gz"
		if len(parsed.args) > 0 {
			outputPath = parsed.args[0]
		}
		if err := runBackup(outputPath); err != nil {
			utils.LogError("[main] backup error: %v\n", err, utils.WithOutput())
			os.Exit(1)
		}
	case "restore":
		archivePath := "/tmp/backup.tar.gz"
		if len(parsed.args) > 0 {
			archivePath = parsed.args[0]
		}
		if err := runRestore(archivePath); err != nil {
			utils.LogError("[main] restore error: %v\n", err, utils.WithOutput())
			os.Exit(1)
		}
	case "mcp-config":
		if err := applyMCPEnabledFromEnv(""); err != nil {
			utils.LogError("[main] mcp-config error: %v\n", err, utils.WithOutput())
			os.Exit(1)
		}
	case "mcp-tokens":
		if err := runMCPTokens(); err != nil {
			utils.LogError("[main] mcp-tokens error: %v\n", err, utils.WithOutput())
			os.Exit(1)
		}
	case "sync":
		if len(parsed.args) < 1 {
			runSyncMode()
			return
		}
		switch parsed.args[0] {
		case "ls":
			s := NewSyncer()
			humanReadable := len(parsed.args) > 1 && parsed.args[1] == "-h"
			if err := s.listSyncFiles(humanReadable); err != nil {
				utils.LogError("[main] sync ls error: %v\n", err, utils.WithOutput())
				os.Exit(1)
			}
		case "rm":
			if len(parsed.args) < 2 {
				utils.LogError("[main] sync rm requires a file argument\n", utils.WithOutput())
				os.Exit(1)
			}
			s := NewSyncer()
			if err := s.removeSyncFile(parsed.args[1]); err != nil {
				utils.LogError("[main] sync rm error: %v\n", err, utils.WithOutput())
				os.Exit(1)
			}
		default:
			utils.LogError("[main] unknown sync subcommand: %s\n", parsed.args[0], utils.WithOutput())
			os.Exit(1)
		}
	case "resync":
		s := NewSyncer()
		if err := s.deleteAllDocuments(); err != nil {
			utils.LogError("[main] resync error: %v\n", err, utils.WithOutput())
			os.Exit(1)
		}
		s.pushAll()
		utils.Log("[main] Resync complete.\n")
	case "zellij-attach":
		if err := runZellijAttach(); err != nil {
			utils.LogError("[main] zellij-attach error: %v\n", err, utils.WithOutput())
			os.Exit(1)
		}
	case "print-env":
		runPrintEnv()
	case "completions":
		handleCompletions(parsed.args)
	case "custom-envs":
		if len(parsed.args) > 0 && parsed.args[0] == "--complete" {
			showCustomEnvsCompletions()
			return
		}
		homeDir, _, _, userID := loadUserConfig()
		if homeDir == "" || userID == "" {
			utils.LogError("[kilo-docker] No user config found\n", utils.WithOutput())
			os.Exit(1)
		}
		if len(parsed.args) < 1 {
			utils.LogError("[kilo-docker] custom-envs requires a subcommand: list, get, add, edit, remove\n", utils.WithOutput())
			os.Exit(1)
		}
		switch parsed.args[0] {
		case "list":
			runCustomEnvsList(homeDir, userID)
		case "get":
			if len(parsed.args) < 2 {
				utils.LogError("[kilo-docker] custom-envs get requires a key\n", utils.WithOutput())
				os.Exit(1)
			}
			runCustomEnvsGet(homeDir, userID, parsed.args[1])
		case "add":
			if len(parsed.args) < 3 {
				utils.LogError("[kilo-docker] custom-envs add requires a key and value\n", utils.WithOutput())
				os.Exit(1)
			}
			runCustomEnvsAdd(homeDir, userID, parsed.args[1], parsed.args[2])
		case "edit":
			if len(parsed.args) < 3 {
				utils.LogError("[kilo-docker] custom-envs edit requires a key and value\n", utils.WithOutput())
				os.Exit(1)
			}
			runCustomEnvsEdit(homeDir, userID, parsed.args[1], parsed.args[2])
		case "remove":
			if len(parsed.args) < 2 {
				utils.LogError("[kilo-docker] custom-envs remove requires a key\n", utils.WithOutput())
				os.Exit(1)
			}
			runCustomEnvsRemove(homeDir, userID, parsed.args[1])
		default:
			utils.LogError("[kilo-docker] unknown custom-envs subcommand: %s\n", parsed.args[0], utils.WithOutput())
			os.Exit(1)
		}
	}
}

// commandWithSubcommand builds a help command key from a command and its args.
// It mirrors the nested-command handling in kilo-docker: commands that accept
// nested subcommands (sync, custom-envs) return "<command> <subcommand>" when
// the first argument is a valid nested subcommand.
func commandWithSubcommand(command string, args []string) string {
	if len(args) == 0 {
		return command
	}
	switch command {
	case "sync":
		if args[0] == "ls" || args[0] == "rm" {
			return command + " " + args[0]
		}
	case "custom-envs":
		if args[0] == "list" || args[0] == "get" || args[0] == "add" || args[0] == "edit" || args[0] == "remove" {
			return command + " " + args[0]
		}
	}
	return command
}
