package main

import (
	"fmt"
	"os"
	"strings"
)

const (
	repoURL      = "ghcr.io/mbabic84/kilo-docker"
	githubRawURL = "https://raw.githubusercontent.com/mbabic84/kilo-docker/main/scripts/kilo-docker"
	kiloHome     = "/home/kilo-t8x3m7kp"
)

var version = "dev"
var autoConfirm bool

// config holds parsed CLI flags for the host binary.
type config struct {
	once        bool
	encrypted   bool
	ainstruct   bool
	playwright  bool
	docker      bool
	zellij      bool
	ssh         bool
	mcp         bool
	yes         bool
	network     string
	networkFlag bool
	command     string
	args        []string
}

// parseFlags parses os.Args into a config struct.
func parseFlags() config {
	var cfg config
	args := os.Args[1:]

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--once":
			cfg.once = true
		case "--password", "-p":
			cfg.encrypted = true
		case "--ainstruct":
			cfg.ainstruct = true
			cfg.encrypted = true
		case "--playwright":
			cfg.playwright = true
		case "--docker":
			cfg.docker = true
		case "--zellij":
			cfg.zellij = true
		case "--ssh":
			cfg.ssh = true
		case "--mcp":
			cfg.mcp = true
		case "--yes", "-y":
			cfg.yes = true
		case "--network":
			cfg.networkFlag = true
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				cfg.network = args[i+1]
				i++
			}
		case "--version":
			fmt.Println(version)
			os.Exit(0)
		default:
			if cfg.command == "" {
				cfg.command = args[i]
			} else {
				cfg.args = append(cfg.args, args[i])
			}
		}
	}

	return cfg
}
