package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mbabic84/kilo-docker/pkg/constants"
	"github.com/mbabic84/kilo-docker/pkg/utils"
)

// profileFlags holds the boolean service/feature flags stored in a profile.
// Services use omitempty so absent fields default to false.
type profileFlags struct {
	Go        bool     `json:"go,omitempty"`
	Docker    bool     `json:"docker,omitempty"`
	UV        bool     `json:"uv,omitempty"`
	GH        bool     `json:"gh,omitempty"`
	SSH       bool     `json:"ssh,omitempty"`
	NVM       bool     `json:"nvm,omitempty"`
	Build     bool     `json:"build,omitempty"`
	Rclone    bool     `json:"rclone,omitempty"`
	Gitnexus  bool     `json:"gitnexus,omitempty"`
	Picomamba bool     `json:"picomamba,omitempty"`
}

// Profile represents a named set of reusable CLI flags stored as JSON under
// ~/.config/kilo-docker/profiles/<name>.json.
type Profile struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Flags       profileFlags `json:"flags,omitempty"`
	Networks    []string     `json:"networks,omitempty"`
	Ports       []string     `json:"ports,omitempty"`
	Volumes     []string     `json:"volumes,omitempty"`
}

// profileDir returns the directory where profile JSON files are stored,
// creating it if necessary.
func profileDir() string {
	d := filepath.Join(constants.GetKiloDockerConfigDir(), "profiles")
	_ = os.MkdirAll(d, 0o700)
	return d
}

// profilePath returns the full filesystem path for a named profile JSON file.
func profilePath(name string) string {
	return filepath.Join(profileDir(), name+".json")
}

// defaultProfilePath returns the path to the .default marker file that names
// the currently active default profile.
func defaultProfilePath() string {
	return filepath.Join(profileDir(), ".default")
}

// validateProfileName checks that a profile name contains only alphanumeric
// characters, dots, underscores, and hyphens, and is non-empty.
func validateProfileName(name string) error {
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-' {
			continue
		}
		return fmt.Errorf("invalid character '%c' in profile name; allowed: a-z, A-Z, 0-9, ., _, -", c)
	}
	return nil
}

// loadProfile reads and deserializes a named profile from disk.
func loadProfile(name string) (Profile, error) {
	var p Profile
	data, err := os.ReadFile(profilePath(name))
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return p, fmt.Errorf("invalid profile JSON: %w", err)
	}
	return p, nil
}

// saveProfile validates and writes a profile to disk with 0600 permissions.
func saveProfile(p Profile) error {
	if err := validateProfileName(p.Name); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(profilePath(p.Name), append(data, '\n'), 0o600)
}

// getDefaultProfile reads the name of the currently active default profile
// from the .default marker file.
func getDefaultProfile() (string, error) {
	data, err := os.ReadFile(defaultProfilePath())
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(string(data))
	if name == "" {
		return "", fmt.Errorf("default profile file is empty")
	}
	return name, nil
}

// setDefaultProfile marks a profile as the default. The profile must already
// exist.
func setDefaultProfile(name string) error {
	if _, err := loadProfile(name); err != nil {
		return fmt.Errorf("profile '%s' not found", name)
	}
	return os.WriteFile(defaultProfilePath(), []byte(name+"\n"), 0o600)
}

// unsetDefaultProfile removes the default profile marker file.
func unsetDefaultProfile() error {
	return os.Remove(defaultProfilePath())
}

// mergeProfile applies a profile's flags to a config. CLI flags already set
// on the config take precedence: services never get removed, SSH only gets
// enabled if not already set, and ports/volumes/networks are always additive.
func mergeProfile(cfg *config, p Profile) {
	if p.Flags.Go {
		addServiceIfMissing(cfg, "go")
	}
	if p.Flags.Docker {
		addServiceIfMissing(cfg, "docker")
	}
	if p.Flags.UV {
		addServiceIfMissing(cfg, "uv")
	}
	if p.Flags.GH {
		addServiceIfMissing(cfg, "gh")
	}
	if p.Flags.NVM {
		addServiceIfMissing(cfg, "nvm")
	}
	if p.Flags.Build {
		addServiceIfMissing(cfg, "build")
	}
	if p.Flags.Rclone {
		addServiceIfMissing(cfg, "rclone")
	}
	if p.Flags.Gitnexus {
		addServiceIfMissing(cfg, "gitnexus")
	}
	if p.Flags.Picomamba {
		addServiceIfMissing(cfg, "picomamba")
	}
	if p.Flags.SSH && !cfg.ssh {
		cfg.ssh = true
	}

	cfg.networks = append(cfg.networks, p.Networks...)

	cfg.ports = append(cfg.ports, p.Ports...)
	cfg.volumes = append(cfg.volumes, p.Volumes...)
}

// addServiceIfMissing adds a service name to cfg.enabledServices only if it
// isn't already present.
func addServiceIfMissing(cfg *config, name string) {
	for _, s := range cfg.enabledServices {
		if s == name {
			return
		}
	}
	cfg.enabledServices = append(cfg.enabledServices, name)
}

// hasAnyFlags returns true if the config has any user-specified flags set
// (services, ssh, playwright, once, networks, ports, or volumes). Used as
// a heuristic to decide whether to auto-load the default profile.
func hasAnyFlags(cfg config) bool {
	if len(cfg.enabledServices) > 0 {
		return true
	}
	if cfg.ssh {
		return true
	}
	if cfg.playwright {
		return true
	}
	if cfg.once {
		return true
	}
	if len(cfg.networks) > 0 {
		return true
	}
	if len(cfg.ports) > 0 {
		return true
	}
	if len(cfg.volumes) > 0 {
		return true
	}
	return false
}

// handleProfile dispatches the "kilo-docker profile <subcommand>" CLI to the
// appropriate handler: save, list, show, edit, delete, import, export,
// set-default, unset-default, or show-default.
func handleProfile(cfg config) {
	if cfg.help {
		printCommandHelp("profile")
		return
	}

	if len(cfg.args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: kilo-docker profile <save|list|show|edit|delete|import|export|set-default|unset-default|show-default> [args...]\n")
		os.Exit(1)
	}

	sub := cfg.args[0]
	rest := cfg.args[1:]

	switch sub {
	case "save":
		if len(rest) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: kilo-docker profile save <name>\n")
			os.Exit(1)
		}
		runProfileSave(cfg, rest[0])

	case "list":
		runProfileList()

	case "show":
		if len(rest) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: kilo-docker profile show <name>\n")
			os.Exit(1)
		}
		runProfileShow(rest[0])

	case "edit":
		if len(rest) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: kilo-docker profile edit <name>\n")
			os.Exit(1)
		}
		runProfileEdit(rest[0])

	case "delete":
		if len(rest) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: kilo-docker profile delete <name>\n")
			os.Exit(1)
		}
		runProfileDelete(cfg, rest[0])

	case "import":
		if len(rest) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: kilo-docker profile import <file>\n")
			os.Exit(1)
		}
		runProfileImport(rest[0])

	case "export":
		if len(rest) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: kilo-docker profile export <name>\n")
			os.Exit(1)
		}
		runProfileExport(rest[0])

	case "set-default":
		if len(rest) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: kilo-docker profile set-default <name>\n")
			os.Exit(1)
		}
		if err := setDefaultProfile(rest[0]); err != nil {
			utils.LogError("[kilo-docker] Failed to set default profile: %v\n", err, utils.WithOutput())
			os.Exit(1)
		}
		utils.Log("[kilo-docker] Default profile set to '%s'\n", rest[0], utils.WithOutput())

	case "unset-default":
		if err := unsetDefaultProfile(); err != nil {
			if !os.IsNotExist(err) {
				utils.LogError("[kilo-docker] Failed to unset default profile: %v\n", err, utils.WithOutput())
				os.Exit(1)
			}
		}
		utils.Log("[kilo-docker] Default profile removed\n", utils.WithOutput())

	case "show-default":
		if name, err := getDefaultProfile(); err == nil {
			utils.Log("[kilo-docker] Default profile: %s\n", name, utils.WithOutput())
		} else {
			utils.Log("[kilo-docker] No default profile set\n", utils.WithOutput())
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown profile subcommand: %s\n", sub)
		fmt.Fprintf(os.Stderr, "Usage: kilo-docker profile <save|list|show|edit|delete|import|export|set-default|unset-default|show-default>\n")
		os.Exit(1)
	}
}

// runProfileSave captures the current config flags and saves them as a named
// profile. The implicit kilo-shared network is filtered out.
func runProfileSave(cfg config, name string) {
	if err := validateProfileName(name); err != nil {
		utils.LogError("[kilo-docker] Invalid profile name: %v\n", err, utils.WithOutput())
		os.Exit(1)
	}

	p := Profile{
		Name: name,
		Flags: profileFlags{
			SSH:       cfg.ssh,
			Picomamba: isServiceEnabled(cfg, "picomamba"),
		},
	}

	for _, svc := range cfg.enabledServices {
		switch svc {
		case "go":
			p.Flags.Go = true
		case "docker":
			p.Flags.Docker = true
		case "uv":
			p.Flags.UV = true
		case "gh":
			p.Flags.GH = true
		case "nvm":
			p.Flags.NVM = true
		case "build":
			p.Flags.Build = true
		case "rclone":
			p.Flags.Rclone = true
		case "gitnexus":
			p.Flags.Gitnexus = true
		}
	}

	p.Networks = cfg.networks
	p.Ports = cfg.ports
	p.Volumes = cfg.volumes

	// Remove kilo-shared from saved networks (it's implicit)
	filtered := make([]string, 0, len(p.Networks))
	for _, n := range p.Networks {
		if n != SharedNetworkName {
			filtered = append(filtered, n)
		}
	}
	p.Networks = filtered

	if err := saveProfile(p); err != nil {
		utils.LogError("[kilo-docker] Failed to save profile: %v\n", err, utils.WithOutput())
		os.Exit(1)
	}
	utils.Log("[kilo-docker] Profile '%s' saved\n", name, utils.WithOutput())
}

// runProfileList prints all saved profiles with name and description,
// marking the default profile with an asterisk.
func runProfileList() {
	entries, err := os.ReadDir(profileDir())
	if err != nil {
		utils.LogError("[kilo-docker] Failed to list profiles: %v\n", err, utils.WithOutput())
		os.Exit(1)
	}

	defaultName, _ := getDefaultProfile()

	var profiles []Profile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		p, err := loadProfile(name)
		if err != nil {
			continue
		}
		profiles = append(profiles, p)
	}

	if len(profiles) == 0 {
		utils.Log("[kilo-docker] No profiles found\n", utils.WithOutput())
		return
	}

	for _, p := range profiles {
		marker := "  "
		if p.Name == defaultName {
			marker = "* "
		}
		desc := p.Description
		if desc != "" {
			desc = " - " + desc
		}
		utils.Log("%s%-20s%s\n", marker, p.Name, desc, utils.WithOutput())
	}
}

// runProfileShow prints the full JSON of a named profile to stdout.
func runProfileShow(name string) {
	p, err := loadProfile(name)
	if err != nil {
		utils.LogError("[kilo-docker] Profile '%s' not found: %v\n", name, err, utils.WithOutput())
		os.Exit(1)
	}
	data, _ := json.MarshalIndent(p, "", "  ")
	fmt.Println(string(data))
}

// runProfileEdit opens a profile in $EDITOR (falling back to vi) for manual
// editing.
func runProfileEdit(name string) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	path := profilePath(name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		utils.LogError("[kilo-docker] Profile '%s' not found\n", name, utils.WithOutput())
		os.Exit(1)
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		utils.LogError("[kilo-docker] Editor failed: %v\n", err, utils.WithOutput())
		os.Exit(1)
	}
}

// runProfileDelete removes a profile after confirmation. Prompts unless --yes
// is set.
func runProfileDelete(cfg config, name string) {
	p, err := loadProfile(name)
	if err != nil {
		utils.LogError("[kilo-docker] Profile '%s' not found\n", name, utils.WithOutput())
		os.Exit(1)
	}

	defaultName, _ := getDefaultProfile()
	if name == defaultName {
		utils.Log("[kilo-docker] Profile '%s' is currently the default\n", name, utils.WithOutput())
	}

	if !cfg.yes {
		if !promptConfirm(fmt.Sprintf("Delete profile '%s' (%s)? [y/N]: ", p.Name, p.Description), false) {
			utils.Log("[kilo-docker] Cancelled\n", utils.WithOutput())
			return
		}
	}

	if err := os.Remove(profilePath(name)); err != nil {
		utils.LogError("[kilo-docker] Failed to delete profile: %v\n", err, utils.WithOutput())
		os.Exit(1)
	}
	utils.Log("[kilo-docker] Profile '%s' deleted\n", name, utils.WithOutput())
}

// runProfileImport reads a JSON file from disk and saves it as a profile.
func runProfileImport(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		utils.LogError("[kilo-docker] Failed to read file: %v\n", err, utils.WithOutput())
		os.Exit(1)
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		utils.LogError("[kilo-docker] Invalid profile JSON: %v\n", err, utils.WithOutput())
		os.Exit(1)
	}
	if err := validateProfileName(p.Name); err != nil {
		utils.LogError("[kilo-docker] Invalid profile name: %v\n", err, utils.WithOutput())
		os.Exit(1)
	}
	if err := saveProfile(p); err != nil {
		utils.LogError("[kilo-docker] Failed to save profile: %v\n", err, utils.WithOutput())
		os.Exit(1)
	}
	utils.Log("[kilo-docker] Profile '%s' imported from %s\n", p.Name, path, utils.WithOutput())
}

// runProfileExport prints a profile's JSON to stdout.
func runProfileExport(name string) {
	p, err := loadProfile(name)
	if err != nil {
		utils.LogError("[kilo-docker] Profile '%s' not found\n", name, utils.WithOutput())
		os.Exit(1)
	}
	data, _ := json.MarshalIndent(p, "", "  ")
	fmt.Println(string(data))
}
