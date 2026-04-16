package main

import (
	"fmt"
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

type config struct {
	once                    bool
	remember                bool
	playwright              bool
	playwrightRecreateVolume bool
	ssh                     bool
	yes                     bool
	help                    bool
	networks                []string
	networkFlag             bool
	ports                   []string
	volumes                 []string
	workspace               string
	command                 string
	args                    []string
	enabledServices         []string
}

type boolFlag struct {
	Names       []string
	Description string
	setField    func(*config)
	serialize   func(config) (string, bool)
}

type valueFlag struct {
	Names       []string
	Description string
	setField    func(*config, string)
	serializeArgs func(config) []string
	buildDockerArgs func(config) []string
}

var boolFlags = []boolFlag{
	{
		Names:       []string{"--help", "-h"},
		Description: "Show help message",
		setField:    func(c *config) { c.help = true },
		serialize:   func(c config) (string, bool) { return "", false },
	},
	{
		Names:       []string{"--once"},
		Description: "Run a one-time session without persisting data (no volume)",
		setField:    func(c *config) { c.once = true },
		serialize:   func(c config) (string, bool) { return "--once", c.once },
	},
	{
		Names:       []string{"--remember"},
		Description: "Remember Ainstruct login for auto-login on future sessions",
		setField:    func(c *config) { c.remember = true },
		serialize:   func(c config) (string, bool) { return "--remember", c.remember },
	},
	{
		Names:       []string{"--playwright"},
		Description: "Start a Playwright MCP sidecar container for browser automation",
		setField:    func(c *config) { c.playwright = true },
		serialize:   func(c config) (string, bool) { return "--playwright", c.playwright },
	},
	{
		Names:       []string{"--ssh"},
		Description: "Enable SSH agent forwarding into the container",
		setField:    func(c *config) { c.ssh = true },
		serialize:   func(c config) (string, bool) { return "--ssh", c.ssh },
	},
	{
		Names:       []string{"--volume", "-v"},
		Description: "Recreate the Playwright volume (delete and create new)",
		setField:    func(c *config) { c.playwrightRecreateVolume = true },
		serialize:   func(c config) (string, bool) { return "--volume", c.playwrightRecreateVolume },
	},
	{
		Names:       []string{"--yes", "-y"},
		Description: "Auto-confirm all prompts (useful for piped/non-interactive installs)",
		setField:    func(c *config) { c.yes = true },
		serialize:   func(c config) (string, bool) { return "--yes", c.yes },
	},
}

var valueFlags = []valueFlag{
	{
		Names:            []string{"--port", "-p"},
		Description:     "Map a port (host_port:container_port). Can be specified multiple times",
		setField:        func(c *config, v string) { c.ports = append(c.ports, v) },
		serializeArgs:   func(c config) []string { return flatten("--port", c.ports) },
		buildDockerArgs: func(c config) []string { return flatten("-p", c.ports) },
	},
	{
		Names:            []string{"--volume", "-v"},
		Description:     "Mount a volume (host_path:container_path). Can be specified multiple times",
		setField:        func(c *config, v string) { c.volumes = append(c.volumes, v) },
		serializeArgs:   func(c config) []string { return flatten("--volume", c.volumes) },
		buildDockerArgs: func(c config) []string { return flatten("-v", c.volumes) },
	},
	{
		Names:            []string{"--workspace", "-w"},
		Description:     "Specify a custom workspace path (defaults to current directory)",
		setField:        func(c *config, v string) { c.workspace = v },
		serializeArgs:   func(c config) []string { return optional("--workspace", c.workspace) },
		buildDockerArgs: func(c config) []string { return nil },
	},
	{
		Names:            []string{"--network"},
		Description:     "Connect to a Docker network (repeatable). 'kilo-shared' is always included.",
		setField:        func(c *config, v string) { c.networks = append(c.networks, v); c.networkFlag = true },
		serializeArgs:   func(c config) []string { return flatten("--network", normalizeNetworks(c.networks, true)) },
		buildDockerArgs: func(c config) []string { return flatten("--network", normalizeNetworks(c.networks, true)) },
	},
}

func flatten(flag string, values []string) []string {
	var result []string
	for _, v := range values {
		result = append(result, flag, v)
	}
	return result
}

func optional(flag, value string) []string {
	if value == "" {
		return nil
	}
	return []string{flag, value}
}

func normalizeForCompare(args string) string {
	parts := strings.Split(args, " ")
	var filtered []string
	skipNext := false
	for i, p := range parts {
		if skipNext {
			skipNext = false
			continue
		}
		if p == "--network" && i+1 < len(parts) && parts[i+1] == SharedNetworkName {
			skipNext = true
			continue
		}
		filtered = append(filtered, p)
	}
	return strings.Join(filtered, " ")
}

func argsMatch(current, stored string) bool {
	return normalizeForCompare(current) == normalizeForCompare(stored)
}

// serializeForDisplay serializes args for display in session list, excluding implicit kilo-shared network.
func serializeForDisplay(cfg config, sshEnabled bool) string {
	var parts []string

	for _, f := range boolFlags {
		if serialized, ok := f.serialize(cfg); ok {
			parts = append(parts, serialized)
		}
	}

	// Only show networks if user explicitly passed --network flag
	// Then exclude kilo-shared from display (it's implicit)
	if cfg.networkFlag {
		var userNetworks []string
		for _, n := range cfg.networks {
			if n != SharedNetworkName {
				userNetworks = append(userNetworks, n)
			}
		}
		if len(userNetworks) > 0 {
			parts = append(parts, flatten("--network", userNetworks)...)
		}
	}

	// Add all other value flags (skip network, it's handled above)
	for _, f := range valueFlags {
		if f.Names[0] == "--network" {
			continue // handled above
		}
		if serialized := f.serializeArgs(cfg); len(serialized) > 0 {
			parts = append(parts, serialized...)
		}
	}

	for _, svcName := range cfg.enabledServices {
		svc := getService(svcName)
		if svc != nil && svc.Flag != "" {
			parts = append(parts, svc.Flag)
		}
	}

	if sshEnabled && !cfg.ssh {
		parts = append(parts, "--ssh")
	}

	if len(cfg.args) > 0 {
		parts = append(parts, cfg.args...)
	}

	return strings.Join(parts, " ")
}

func parseArgs(args []string) config {
	var cfg config

	for i := 0; i < len(args); i++ {
		arg := args[i]
		consumed := false

		for _, f := range boolFlags {
			for _, name := range f.Names {
				if arg == name {
					f.setField(&cfg)
					consumed = true
					break
				}
			}
			if consumed {
				break
			}
		}

		if !consumed {
			for _, f := range valueFlags {
				for _, name := range f.Names {
					if arg == name {
						if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
							f.setField(&cfg, args[i+1])
							i++
						}
						consumed = true
						break
					}
				}
				if consumed {
					break
				}
			}
		}

		if !consumed {
			for _, svc := range services.BuiltInServices {
				if arg == svc.Flag {
					cfg.enabledServices = append(cfg.enabledServices, svc.Name)
					consumed = true
					break
				}
			}
		}

		if !consumed {
			if cfg.command == "" {
				cfg.command = arg
			} else {
				cfg.args = append(cfg.args, arg)
			}
		}
	}

	return cfg
}

func parseFlags() config {
	return parseArgs(os.Args[1:])
}

func formatFlagHelp() string {
	var lines []string
	for _, f := range boolFlags {
		lines = append(lines, fmt.Sprintf("  %-*s %s", 40, f.Names[0], f.Description))
	}
	for _, f := range valueFlags {
		lines = append(lines, fmt.Sprintf("  %-*s %s", 40, f.Names[0]+" <value>", f.Description))
	}
	return strings.Join(lines, "\n")
}
