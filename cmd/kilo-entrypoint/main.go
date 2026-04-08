// kilo-entrypoint is the container entrypoint binary for the kilo-docker image.
//
// When invoked with no arguments (or as the default ENTRYPOINT), it performs
// container initialization: user/group setup, tool downloads, SSH known_hosts,
// config directory creation, privilege drop, and MCP server toggling.
//
// When invoked with a subcommand, it delegates to the appropriate handler:
//
//	ainstruct-login Authenticate with Ainstruct API, output structured result
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
	"syscall"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

// subcommands lists the internal subcommands handled by kilo-entrypoint.
// Any argument NOT in this map is passed through to exec.LookPath for
// direct binary execution (e.g. "kilo", "sh", "bash").
var subcommands = map[string]bool{
	"ainstruct-login": true,
	"update-config":   true,
	"backup":          true,
	"restore":         true,
	"mcp-config":      true,
	"mcp-tokens":      true,
	"sync":            true,
	"resync":          true,
	"zellij-attach":   true,
	"print-env":       true,
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
	fmt.Printf("  %-*s %s\n", w, "ainstruct-login", "Authenticate with Ainstruct API, output structured result")
	fmt.Printf("  %-*s %s\n", w, "update-config", "Download config template, merge with existing config")
	fmt.Printf("  %-*s %s\n", w, "backup [path]", "Create tar.gz of KILO_HOME (default: /tmp/backup.tar.gz)")
	fmt.Printf("  %-*s %s\n", w, "restore [path]", "Extract tar.gz into KILO_HOME with ownership fix")
		fmt.Printf("  %-*s %s\n", w, "mcp-config", "Apply MCP enabled states from encrypted token storage")
	fmt.Printf("  %-*s %s\n", w, "mcp-tokens", "Interactive token management")
	fmt.Printf("  %-*s %s\n", w, "sync", "Start ainstruct file watcher + REST sync")
	fmt.Printf("  %-*s %s\n", w, "resync", "Delete all remote documents and re-push local files")
	fmt.Printf("  %-*s %s\n", w, "zellij-attach", "Attach to existing zellij session")
	fmt.Printf("  %-*s %s\n", w, "print-env", "Print export statements for current tokens")
	fmt.Println("")
	fmt.Println("Any other argument is passed through to exec.LookPath for")
	fmt.Println("direct binary execution (e.g. \"kilo\", \"sh\", \"bash\").")
}

func runPrintEnv() {
	homeDir, _, _, userID := loadUserConfig()
	if homeDir == "" || userID == "" {
		return
	}

	context7, ainstruct, _, _, _, err := loadEncryptedTokens(homeDir, userID)
	if err != nil {
		return
	}

	fmt.Printf("export KD_MCP_CONTEXT7_TOKEN=%q\n", context7)
	fmt.Printf("export KD_MCP_AINSTRUCT_TOKEN=%q\n", ainstruct)
}

func main() {
	if len(os.Args) < 2 {
		if err := runInit(); err != nil {
			utils.LogError("[main] init error: %v\n", err, utils.WithOutput())
			os.Exit(1)
		}
		return
	}

	name := os.Args[1]
	binary, passthrough := resolveCommand(name)
	if passthrough {
		if binary == "" {
			utils.LogError("[main] unknown subcommand or command: %s\n", name, utils.WithOutput())
			os.Exit(1)
		}
		if err := syscall.Exec(binary, os.Args[1:], os.Environ()); err != nil {
			utils.LogError("[main] exec %s: %v\n", name, err, utils.WithOutput())
			os.Exit(1)
		}
		return
	}

	switch name {
	case "ainstruct-login":
		if err := runAinstructLogin(); err != nil {
			utils.LogError("[main] STATUS=error\nERROR=%v\n", err, utils.WithOutput())
			os.Exit(1)
		}
	case "update-config":
		if err := runUpdateConfig(); err != nil {
			utils.LogError("[main] update-config error: %v\n", err, utils.WithOutput())
			os.Exit(1)
		}
	case "backup":
		outputPath := "/tmp/backup.tar.gz"
		if len(os.Args) > 2 {
			outputPath = os.Args[2]
		}
		if err := runBackup(outputPath); err != nil {
			utils.LogError("[main] backup error: %v\n", err, utils.WithOutput())
			os.Exit(1)
		}
	case "restore":
		archivePath := "/tmp/backup.tar.gz"
		if len(os.Args) > 2 {
			archivePath = os.Args[2]
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
		runSyncMode()
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
	case "help":
		runHelp()
	}
}
