// kilo-entrypoint is the container entrypoint binary for the kilo-docker image.
//
// When invoked with no arguments (or as the default ENTRYPOINT), it performs
// container initialization: user/group setup, tool downloads, SSH known_hosts,
// config directory creation, privilege drop, and MCP server toggling.
//
// When invoked with a subcommand, it delegates to the appropriate handler:
//
//	load-tokens     Read token env file, output KEY=VALUE to stdout
//	save-tokens     Read KEY=VALUE from stdin, write to token file
//	ainstruct-login Authenticate with Ainstruct API, output structured result
//	update-config   Download config template, merge with existing config
//	backup          Create tar.gz of KILO_HOME
//	restore         Extract tar.gz into KILO_HOME with ownership fix
//	config          Toggle MCP servers based on environment variables
//	sync            Start ainstruct file watcher + REST sync
//	resync          Delete all remote documents and re-push local files
package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// subcommands lists the internal subcommands handled by kilo-entrypoint.
// Any argument NOT in this map is passed through to exec.LookPath for
// direct binary execution (e.g. "kilo", "sh", "bash").
var subcommands = map[string]bool{
	"load-tokens":     true,
	"save-tokens":     true,
	"ainstruct-login": true,
	"update-config":   true,
	"backup":          true,
	"restore":         true,
	"config":          true,
	"sync":            true,
	"resync":          true,
	"zellij-attach":   true,
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
	fmt.Printf("  %-*s %s\n", w, "load-tokens", "Read token env file, output KEY=VALUE to stdout")
	fmt.Printf("  %-*s %s\n", w, "save-tokens", "Read KEY=VALUE from stdin, write to token file")
	fmt.Printf("  %-*s %s\n", w, "ainstruct-login", "Authenticate with Ainstruct API, output structured result")
	fmt.Printf("  %-*s %s\n", w, "update-config", "Download config template, merge with existing config")
	fmt.Printf("  %-*s %s\n", w, "backup [path]", "Create tar.gz of KILO_HOME (default: /tmp/backup.tar.gz)")
	fmt.Printf("  %-*s %s\n", w, "restore [path]", "Extract tar.gz into KILO_HOME with ownership fix")
	fmt.Printf("  %-*s %s\n", w, "config", "Toggle MCP servers based on environment variables")
	fmt.Printf("  %-*s %s\n", w, "sync", "Start ainstruct file watcher + REST sync")
	fmt.Printf("  %-*s %s\n", w, "resync", "Delete all remote documents and re-push local files")
	fmt.Printf("  %-*s %s\n", w, "zellij-attach", "Attach to existing zellij session")
	fmt.Println("")
	fmt.Println("Any other argument is passed through to exec.LookPath for")
	fmt.Println("direct binary execution (e.g. \"kilo\", \"sh\", \"bash\").")
}

func main() {
	if len(os.Args) < 2 {
		if err := runInit(); err != nil {
			fmt.Fprintf(os.Stderr, "init error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	name := os.Args[1]
	binary, passthrough := resolveCommand(name)
	if passthrough {
		if binary == "" {
			fmt.Fprintf(os.Stderr, "unknown subcommand or command: %s\n", name)
			os.Exit(1)
		}
		if err := syscall.Exec(binary, os.Args[1:], os.Environ()); err != nil {
			fmt.Fprintf(os.Stderr, "exec %s: %v\n", name, err)
			os.Exit(1)
		}
		return
	}

	switch name {
	case "load-tokens":
		if err := runLoadTokens(); err != nil {
			fmt.Fprintf(os.Stderr, "load-tokens error: %v\n", err)
			os.Exit(1)
		}
	case "save-tokens":
		if err := runSaveTokens(); err != nil {
			fmt.Fprintf(os.Stderr, "save-tokens error: %v\n", err)
			os.Exit(1)
		}
	case "ainstruct-login":
		if err := runAinstructLogin(); err != nil {
			fmt.Fprintf(os.Stderr, "STATUS=error\nERROR=%v\n", err)
			os.Exit(1)
		}
	case "update-config":
		if err := runUpdateConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "update-config error: %v\n", err)
			os.Exit(1)
		}
	case "backup":
		outputPath := "/tmp/backup.tar.gz"
		if len(os.Args) > 2 {
			outputPath = os.Args[2]
		}
		if err := runBackup(outputPath); err != nil {
			fmt.Fprintf(os.Stderr, "backup error: %v\n", err)
			os.Exit(1)
		}
	case "restore":
		archivePath := "/tmp/backup.tar.gz"
		if len(os.Args) > 2 {
			archivePath = os.Args[2]
		}
		if err := runRestore(archivePath); err != nil {
			fmt.Fprintf(os.Stderr, "restore error: %v\n", err)
			os.Exit(1)
		}
	case "config":
		if err := runConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "config error: %v\n", err)
			os.Exit(1)
		}
	case "sync":
		runSyncMode()
	case "resync":
		s := NewSyncer()
		if err := s.deleteAllDocuments(); err != nil {
			fmt.Fprintf(os.Stderr, "resync error: %v\n", err)
			os.Exit(1)
		}
		s.pushAll()
		fmt.Println("Resync complete.")
	case "zellij-attach":
		if err := runZellijAttach(); err != nil {
			fmt.Fprintf(os.Stderr, "zellij-attach error: %v\n", err)
			os.Exit(1)
		}
	case "help":
		runHelp()
	}
}
