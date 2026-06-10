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
	"flag"
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
	"help":            true,
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

func runHelp() {
	const w = 40
	fmt.Println("kilo-entrypoint - Container entrypoint for kilo-docker")
	fmt.Println("")
	fmt.Println("Usage: kilo-entrypoint [subcommand]")
	fmt.Println("")
	fmt.Println("With no arguments, runs container initialization.")
	fmt.Println("")
	fmt.Println("Subcommands:")
	fmt.Printf("  %-*s %s\n", w, "help", "Show this help message")
	fmt.Printf("  %-*s %s\n", w, "update-config", "Download config template, merge with existing config")
	fmt.Printf("  %-*s %s\n", w, "backup [path]", "Create tar.gz of KILO_HOME (default: /tmp/backup.tar.gz)")
	fmt.Printf("  %-*s %s\n", w, "restore [path]", "Extract tar.gz into KILO_HOME with ownership fix")
	fmt.Printf("  %-*s %s\n", w, "mcp-config", "Apply MCP enabled states from encrypted token storage")
	fmt.Printf("  %-*s %s\n", w, "mcp-tokens", "Interactive token management")
	fmt.Printf("  %-*s %s\n", w, "sync", "Start ainstruct file watcher + REST sync")
	fmt.Printf("  %-*s %s\n", w, "sync ls", "List all ainstruct sync files")
	fmt.Printf("  %-*s %s\n", w, "sync rm <file>", "Remove a specific sync file (local and remote)")
	fmt.Printf("  %-*s %s\n", w, "resync", "Delete all remote documents and re-push local files")
	fmt.Printf("  %-*s %s\n", w, "zellij-attach", "Attach to existing zellij session")
	fmt.Printf("  %-*s %s\n", w, "print-env", "Print export statements for current tokens and custom envs")
	fmt.Printf("  %-*s %s\n", w, "custom-envs", "Manage user-defined custom environment variables")
	fmt.Println("")
	fmt.Println("Custom Envs Subcommands:")
	fmt.Printf("  %-*s %s\n", w, "custom-envs list", "List all custom envs (keys + masked values)")
	fmt.Printf("  %-*s %s\n", w, "custom-envs get <key>", "Print raw value of a custom env to stdout")
	fmt.Printf("  %-*s %s\n", w, "custom-envs add <key> <value>", "Add a new custom env")
	fmt.Printf("  %-*s %s\n", w, "custom-envs edit <key> <value>", "Edit an existing custom env")
	fmt.Printf("  %-*s %s\n", w, "custom-envs remove <key>", "Remove a custom env")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  kilo-entrypoint zellij-attach")
	fmt.Println("  kilo-entrypoint sync")
	fmt.Println("")
	fmt.Println("Any other argument is passed through to exec.LookPath for")
	fmt.Println("direct binary execution (e.g. \"kilo\", \"sh\", \"bash\").")
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
	flag.Parse()
	utils.Log("[main] Entrypoint started, os.Args=%v\n", os.Args)

	remaining := flag.Args()
	hasSubcommand := len(remaining) > 0 && !strings.HasPrefix(remaining[0], "-")
	onlyFlags := len(remaining) == 0 || (len(remaining) == 1 && strings.HasPrefix(remaining[0], "-"))
	alreadyInitialized := func() bool {
		_, err := os.Stat("/tmp/.kilo-initialized")
		return err == nil
	}

	if onlyFlags && !hasSubcommand {
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

	name := remaining[0]
	binary, passthrough := resolveCommand(name)
	if passthrough {
		if binary == "" {
			utils.LogError("[main] unknown subcommand or command: %s\n", name, utils.WithOutput())
			os.Exit(1)
		}
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
		if len(remaining) > 1 {
			outputPath = remaining[1]
		}
		if err := runBackup(outputPath); err != nil {
			utils.LogError("[main] backup error: %v\n", err, utils.WithOutput())
			os.Exit(1)
		}
	case "restore":
		archivePath := "/tmp/backup.tar.gz"
		if len(remaining) > 1 {
			archivePath = remaining[1]
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
		if len(remaining) < 2 {
			runSyncMode()
			return
		}
		switch remaining[1] {
		case "ls":
			s := NewSyncer()
			humanReadable := len(remaining) > 2 && remaining[2] == "-h"
			if err := s.listSyncFiles(humanReadable); err != nil {
				utils.LogError("[main] sync ls error: %v\n", err, utils.WithOutput())
				os.Exit(1)
			}
		case "rm":
			if len(remaining) < 3 {
				utils.LogError("[main] sync rm requires a file argument\n", utils.WithOutput())
				os.Exit(1)
			}
			s := NewSyncer()
			if err := s.removeSyncFile(remaining[2]); err != nil {
				utils.LogError("[main] sync rm error: %v\n", err, utils.WithOutput())
				os.Exit(1)
			}
		default:
			utils.LogError("[main] unknown sync subcommand: %s\n", remaining[1], utils.WithOutput())
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
	case "custom-envs":
		homeDir, _, _, userID := loadUserConfig()
		if homeDir == "" || userID == "" {
			utils.LogError("[kilo-docker] No user config found\n", utils.WithOutput())
			os.Exit(1)
		}
		if len(remaining) < 2 {
			utils.LogError("[kilo-docker] custom-envs requires a subcommand: list, get, add, edit, remove\n", utils.WithOutput())
			os.Exit(1)
		}
		switch remaining[1] {
		case "list":
			runCustomEnvsList(homeDir, userID)
		case "get":
			if len(remaining) < 3 {
				utils.LogError("[kilo-docker] custom-envs get requires a key\n", utils.WithOutput())
				os.Exit(1)
			}
			runCustomEnvsGet(homeDir, userID, remaining[2])
		case "add":
			if len(remaining) < 4 {
				utils.LogError("[kilo-docker] custom-envs add requires a key and value\n", utils.WithOutput())
				os.Exit(1)
			}
			runCustomEnvsAdd(homeDir, userID, remaining[2], remaining[3])
		case "edit":
			if len(remaining) < 4 {
				utils.LogError("[kilo-docker] custom-envs edit requires a key and value\n", utils.WithOutput())
				os.Exit(1)
			}
			runCustomEnvsEdit(homeDir, userID, remaining[2], remaining[3])
		case "remove":
			if len(remaining) < 3 {
				utils.LogError("[kilo-docker] custom-envs remove requires a key\n", utils.WithOutput())
				os.Exit(1)
			}
			runCustomEnvsRemove(homeDir, userID, remaining[2])
		default:
			utils.LogError("[kilo-docker] unknown custom-envs subcommand: %s\n", remaining[1], utils.WithOutput())
			os.Exit(1)
		}
	case "help":
		runHelp()
	}
}
