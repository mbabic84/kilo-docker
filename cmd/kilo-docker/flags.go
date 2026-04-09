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
	once            bool
	remember        bool
	playwright      bool
	ssh             bool
	yes             bool
	network         string
	networkFlag     bool
	ports           []string
	volumes         []string
	workspace       string
	command         string
	args            []string
	enabledServices []string
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
		Description:     "Connect the container to a Docker network",
		setField:        func(c *config, v string) { c.network = v; c.networkFlag = true },
		serializeArgs:   func(c config) []string { return optional("--network", c.network) },
		buildDockerArgs: func(c config) []string { return optional("--network", c.network) },
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
