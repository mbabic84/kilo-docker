package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateProfileName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"fullstack", false},
		{"my-profile", false},
		{"test.profile", false},
		{"dev_env", false},
		{"Foo123", false},
		{"123abc", false},
		{"", true},
		{"has space", true},
		{"path/traversal", true},
		{"../escape", true},
		{"semicolon;", true},
		{"has!bang", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProfileName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateProfileName(%q) error = %v, wantErr = %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestSaveProfileRoundTrip(t *testing.T) {
	d := filepath.Join(t.TempDir(), "profiles")
	_ = os.MkdirAll(d, 0o700)

	p := Profile{
		Name:        "roundtrip",
		Description: "Round trip test",
		Flags:       profileFlags{Go: true, SSH: true},
		Networks:    []string{"dev"},
		Ports:       []string{"8080:80"},
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	err = os.WriteFile(filepath.Join(d, "roundtrip.json"), append(data, '\n'), 0o600)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	var loaded Profile
	loadedData, err := os.ReadFile(filepath.Join(d, "roundtrip.json"))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if err := json.Unmarshal(loadedData, &loaded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if loaded.Name != p.Name {
		t.Errorf("expected name %q, got %q", p.Name, loaded.Name)
	}
	if loaded.Description != p.Description {
		t.Errorf("expected description %q, got %q", p.Description, loaded.Description)
	}
	if !loaded.Flags.Go {
		t.Error("expected Go flag to be true")
	}
	if !loaded.Flags.SSH {
		t.Error("expected SSH flag to be true")
	}
	if len(loaded.Networks) != 1 || loaded.Networks[0] != "dev" {
		t.Errorf("expected networks [dev], got %v", loaded.Networks)
	}
}

func TestMergeProfileServices(t *testing.T) {
	cfg := &config{}
	p := Profile{
		Flags: profileFlags{Go: true, Docker: true, UV: true},
	}

	mergeProfile(cfg, p)

	if !isServiceEnabled(*cfg, "go") {
		t.Error("expected go service")
	}
	if !isServiceEnabled(*cfg, "docker") {
		t.Error("expected docker service")
	}
	if !isServiceEnabled(*cfg, "uv") {
		t.Error("expected uv service")
	}
}

func TestMergeProfileCLIOverridesProfile(t *testing.T) {
	cfg := &config{
		enabledServices: []string{"go"},
		ssh:             true,
	}
	p := Profile{
		Flags: profileFlags{Go: true, SSH: true, Docker: true},
	}

	mergeProfile(cfg, p)

	if !isServiceEnabled(*cfg, "go") {
		t.Error("expected go service")
	}
	if !isServiceEnabled(*cfg, "docker") {
		t.Error("expected docker service (additive)")
	}
	if !cfg.ssh {
		t.Error("expected ssh flag")
	}
}

func TestMergeProfileNetworks(t *testing.T) {
	cfg := &config{
		networks: []string{"existing"},
	}
	p := Profile{
		Networks: []string{"profile-net"},
	}

	mergeProfile(cfg, p)

	if len(cfg.networks) != 1 || cfg.networks[0] != "existing" {
		t.Errorf("expected networks [existing], got %v", cfg.networks)
	}
}

func TestMergeProfileNetworksEmptyCLI(t *testing.T) {
	cfg := &config{}
	p := Profile{
		Networks: []string{"profile-net"},
	}

	mergeProfile(cfg, p)

	if len(cfg.networks) != 1 || cfg.networks[0] != "profile-net" {
		t.Errorf("expected networks [profile-net], got %v", cfg.networks)
	}
}

func TestMergeProfilePortsAdditive(t *testing.T) {
	cfg := &config{
		ports: []string{"8080:80"},
	}
	p := Profile{
		Ports: []string{"3000:3000"},
	}

	mergeProfile(cfg, p)

	if len(cfg.ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(cfg.ports))
	}
	if cfg.ports[0] != "8080:80" || cfg.ports[1] != "3000:3000" {
		t.Errorf("expected [8080:80 3000:3000], got %v", cfg.ports)
	}
}

func TestMergeProfileDoesNotSetWorkspace(t *testing.T) {
	cfg := &config{
		workspace: "/my/custom/path",
	}
	p := Profile{}

	mergeProfile(cfg, p)

	if cfg.workspace != "/my/custom/path" {
		t.Errorf("expected workspace to remain, got %q", cfg.workspace)
	}
}

func TestHasAnyFlags(t *testing.T) {
	if hasAnyFlags(config{}) {
		t.Error("empty config should have no flags")
	}

	if !hasAnyFlags(config{enabledServices: []string{"go"}}) {
		t.Error("config with service should have flags")
	}

	if !hasAnyFlags(config{ssh: true}) {
		t.Error("config with ssh should have flags")
	}

	if !hasAnyFlags(config{playwright: true}) {
		t.Error("config with playwright should have flags")
	}

	if !hasAnyFlags(config{once: true}) {
		t.Error("config with once should have flags")
	}

	if !hasAnyFlags(config{networks: []string{"net"}}) {
		t.Error("config with network should have flags")
	}

	if !hasAnyFlags(config{ports: []string{"8080:80"}}) {
		t.Error("config with port should have flags")
	}

	if !hasAnyFlags(config{volumes: []string{"/a:/b"}}) {
		t.Error("config with volume should have flags")
	}
}

func TestParseArgsWithProfileFlag(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"kilo-docker", "--profile", "fullstack"}
	cfg := parseArgs(os.Args[1:])

	if cfg.profile != "fullstack" {
		t.Errorf("expected profile = %q, got %q", "fullstack", cfg.profile)
	}
}

func TestSerializeArgsWithProfile(t *testing.T) {
	cfg := config{
		profile: "fullstack",
	}
	result := serializeArgs(cfg, false)
	if result != "--profile fullstack --network kilo-shared" {
		t.Errorf("expected '--profile fullstack --network kilo-shared', got %q", result)
	}
}

func TestAddServiceIfMissing(t *testing.T) {
	cfg := &config{}

	addServiceIfMissing(cfg, "go")
	if len(cfg.enabledServices) != 1 || cfg.enabledServices[0] != "go" {
		t.Errorf("expected [go], got %v", cfg.enabledServices)
	}

	addServiceIfMissing(cfg, "go")
	if len(cfg.enabledServices) != 1 {
		t.Errorf("expected still 1 service, got %d", len(cfg.enabledServices))
	}

	addServiceIfMissing(cfg, "docker")
	if len(cfg.enabledServices) != 2 {
		t.Errorf("expected 2 services, got %d", len(cfg.enabledServices))
	}
}

func TestMergeProfileGHAndNVM(t *testing.T) {
	cfg := &config{}
	p := Profile{
		Flags: profileFlags{GH: true, NVM: true, Build: true, Rclone: true, Gitnexus: true, Picomamba: true},
	}

	mergeProfile(cfg, p)

	if !isServiceEnabled(*cfg, "gh") {
		t.Error("expected gh service")
	}
	if !isServiceEnabled(*cfg, "nvm") {
		t.Error("expected nvm service")
	}
	if !isServiceEnabled(*cfg, "build") {
		t.Error("expected build service")
	}
	if !isServiceEnabled(*cfg, "rclone") {
		t.Error("expected rclone service")
	}
	if !isServiceEnabled(*cfg, "gitnexus") {
		t.Error("expected gitnexus service")
	}
	if !isServiceEnabled(*cfg, "picomamba") {
		t.Error("expected picomamba service")
	}
}
