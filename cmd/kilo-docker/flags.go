package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/mbabic84/kilo-docker/pkg/services"
	"github.com/mbabic84/kilo-docker/pkg/utils"
)

const (
	repoURL  = "ghcr.io/mbabic84/kilo-docker"
	kiloHome = "/home"
)

var version = "dev"
var kiloVersion = "unknown"

type config struct {
	once                     bool
	playwright               bool
	playwrightRecreateVolume bool
	ssh                      bool
	yes                      bool
	help                     bool
	profile                  string
	networks                 []string
	networkFlag              bool
	ports                    []string
	volumes                  []string
	workspace                string
	command                  string
	args                     []string
	enabledServices          []string
}

type boolFlag struct {
	Names       []string
	Description string
	setField    func(*config)
	serialize   func(config) (string, bool)
}

type valueFlag struct {
	Names           []string
	Description     string
	setField        func(*config, string)
	serializeArgs   func(config) []string
	buildDockerArgs func(config) []string
}

func newRepeatableValueFlag(longName, shortName, description string, getValues func(config) []string, setValue func(*config, string)) valueFlag {
	return valueFlag{
		Names:           []string{longName, shortName},
		Description:     description,
		setField:        setValue,
		serializeArgs:   func(c config) []string { return flatten(longName, getValues(c)) },
		buildDockerArgs: func(c config) []string { return flatten(shortName, getValues(c)) },
	}
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
	newRepeatableValueFlag(
		"--port",
		"-p",
		"Map a port (host_port:container_port). Can be specified multiple times",
		func(c config) []string { return c.ports },
		func(c *config, v string) { c.ports = append(c.ports, v) },
	),
	newRepeatableValueFlag(
		"--volume",
		"-v",
		"Mount a volume (host_path:container_path). Can be specified multiple times",
		func(c config) []string { return c.volumes },
		func(c *config, v string) { c.volumes = append(c.volumes, v) },
	),
	{
		Names:           []string{"--workspace", "-w"},
		Description:     "Specify a custom workspace path (defaults to current directory)",
		setField:        func(c *config, v string) { c.workspace = v },
		serializeArgs:   func(c config) []string { return optional("--workspace", c.workspace) },
		buildDockerArgs: func(c config) []string { return nil },
	},
	{
		Names:           []string{"--profile"},
		Description:     "Load a named profile from ~/.config/kilo-docker/profiles/",
		setField:        func(c *config, v string) { c.profile = v },
		serializeArgs:   func(c config) []string { return optional("--profile", c.profile) },
		buildDockerArgs: func(c config) []string { return nil },
	},
	{
		Names:           []string{"--network"},
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
	return comparisonSignature(parseArgs(strings.Fields(args)))
}

func argsMatch(current, stored string) bool {
	return normalizeForCompare(current) == normalizeForCompare(stored)
}

func comparisonSignature(cfg config) string {
	var parts []string
	parts = append(parts,
		strconv.FormatBool(cfg.once),
		strconv.FormatBool(cfg.playwright),
		strconv.FormatBool(cfg.playwrightRecreateVolume),
		strconv.FormatBool(cfg.ssh),
		strconv.FormatBool(cfg.yes),
		strconv.FormatBool(cfg.help),
		cfg.profile,
		cfg.workspace,
		cfg.command,
		joinSorted(cfg.args),
		joinSorted(normalizedNetworks(cfg.networks)),
		joinSorted(cfg.ports),
		joinSorted(cfg.volumes),
		joinSorted(cfg.enabledServices),
	)
	return strings.Join(parts, "\x00")
}

func normalizedNetworks(networks []string) []string {
	var normalized []string
	for _, network := range networks {
		if network != SharedNetworkName {
			normalized = append(normalized, network)
		}
	}
	return normalized
}

func joinSorted(values []string) string {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return strings.Join(sorted, "\x00")
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

func isAmbiguousVolumeValue(name string, index int, args []string) bool {
	if name != "--volume" && name != "-v" {
		return false
	}
	return index+1 < len(args) && !strings.HasPrefix(args[index+1], "--")
}

func parseArgs(args []string) config {
	var cfg config

	for i := 0; i < len(args); i++ {
		arg := args[i]
		consumed := false

		for _, f := range boolFlags {
			for _, name := range f.Names {
				if arg == name && !isAmbiguousVolumeValue(name, i, args) {
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

func mergeOverrides(base, override config) config {
	if override.once {
		base.once = true
	}
	if override.playwright {
		base.playwright = true
	}
	if override.playwrightRecreateVolume {
		base.playwrightRecreateVolume = true
	}
	if override.ssh {
		base.ssh = true
	}
	if override.yes {
		base.yes = true
	}
	if override.help {
		base.help = true
	}

	if override.profile != "" {
		base.profile = override.profile
	}
	if override.workspace != "" {
		base.workspace = override.workspace
	}

	if len(override.ports) > 0 {
		base.ports = override.ports
	}
	if len(override.volumes) > 0 {
		base.volumes = override.volumes
	}
	if override.networkFlag {
		base.networkFlag = true
		base.networks = override.networks
	}
	if len(override.enabledServices) > 0 {
		base.enabledServices = override.enabledServices
	}

	return base
}

func parseFlags() config {
	cfg := parseArgs(os.Args[1:])

	profileName := cfg.profile
	if profileName == "" && !cfg.help && cfg.command == "" && !hasAnyFlags(cfg) {
		if defaultName, err := getDefaultProfile(); err == nil && defaultName != "" {
			profileName = defaultName
			utils.Log("[kilo-docker] Using default profile: %s\n", profileName, utils.WithOutput())
		}
	}

	if profileName != "" {
		p, err := loadProfile(profileName)
		if err != nil {
			utils.LogError("[kilo-docker] Profile '%s' not found: %v\n", profileName, err, utils.WithOutput())
			os.Exit(1)
		}
		mergeProfile(&cfg, p)
	}

	// --network host is incompatible with other networks (Docker restriction)
	if containsNet(cfg.networks, "host") && len(cfg.networks) > 1 {
		var others []string
		for _, n := range cfg.networks {
			if n != "host" {
				others = append(others, n)
			}
		}
		utils.LogWarn("[kilo-docker] --network host cannot be combined with other networks; ignoring %v\n", others, utils.WithOutput())
		cfg.networks = []string{"host"}
	}

	return cfg
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

func serializeStoredArgs(storedArgs string) string {
	if storedArgs == "" {
		return ""
	}
	cfg := parseArgs(strings.Fields(storedArgs))
	return serializeForDisplay(cfg, false)
}
