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
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		if err := runInit(); err != nil {
			fmt.Fprintf(os.Stderr, "init error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	subcommand := os.Args[1]
	switch subcommand {
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
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}
