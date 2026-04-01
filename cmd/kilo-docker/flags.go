package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/kilo-org/kilo-docker/pkg/services"
)

const (
	repoURL  = "ghcr.io/mbabic84/kilo-docker"
	kiloHome = "/home/kilo-t8x3m7kp"
)

var version = "dev"
var kiloVersion = "unknown"
var autoConfirm bool

// config holds parsed CLI flags for the host binary.
type config struct {
	once            bool
	encrypted       bool
	ainstruct       bool
	playwright      bool
	ssh             bool
	mcp             bool
	yes             bool
	network         string
	networkFlag     bool
	command         string
	args            []string
	enabledServices []string // Names of enabled services from builtInServices
}

// parseArgs parses the given arguments into a config struct.
func parseArgs(args []string) config {
	var cfg config

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
			matched := false
			for _, svc := range services.BuiltInServices {
				if args[i] == svc.Flag {
					cfg.enabledServices = append(cfg.enabledServices, svc.Name)
					matched = true
					break
				}
			}
			if !matched {
				if cfg.command == "" {
					cfg.command = args[i]
				} else {
					cfg.args = append(cfg.args, args[i])
				}
			}
		}
	}

	return cfg
}

// parseFlags parses os.Args into a config struct.
func parseFlags() config {
	return parseArgs(os.Args[1:])
}
