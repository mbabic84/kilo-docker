package main

import (
	"os"
	"strings"

	"github.com/mbabic84/kilo-docker/pkg/services"
)

const (
	repoURL  = "ghcr.io/mbabic84/kilo-docker"
	kiloHome = "/home"
)

var version = "dev"
var kiloVersion = "unknown"

// config holds parsed CLI flags for the host binary.
type config struct {
	once            bool
	playwright      bool
	ssh             bool
	yes             bool
	network         string
	networkFlag     bool
	ports           []string // Port mappings in host_port:container_port format
	volumes         []string // Volume mounts in host_path:container_path format
	workspace       string   // Custom workspace path (defaults to pwd)
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
		case "--port", "-p":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				cfg.ports = append(cfg.ports, args[i+1])
				i++
			}
		case "--volume", "-v":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				cfg.volumes = append(cfg.volumes, args[i+1])
				i++
			}
		case "--workspace", "-w":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				cfg.workspace = args[i+1]
				i++
			}
		case "--playwright":
			cfg.playwright = true
		case "--ssh":
			cfg.ssh = true
		case "--yes", "-y":
			cfg.yes = true
		case "--network":
			cfg.networkFlag = true
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				cfg.network = args[i+1]
				i++
			}
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
