package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubInstallCmd replaces the real command runner with a no-op for tests,
// preventing actual package downloads and binary installations.
// Returns a cleanup function that restores the original.
func stubInstallCmd() func() {
	orig := runInstallCmd
	runInstallCmd = func(cmd string) error { return nil }
	return func() { runInstallCmd = orig }
}

func TestExpandHome(t *testing.T) {
	tests := []struct {
		path     string
		home     string
		expected string
	}{
		{"~/.config/file", "/home/user", "/home/user/.config/file"},
		{"~/file", "/home/user", "/home/user/file"},
		{"/absolute/path", "/home/user", "/absolute/path"},
		{"", "/home/user", ""},
		{"no-tilde", "/home/user", "no-tilde"},
	}

	for _, tt := range tests {
		result := expandHome(tt.path, tt.home)
		if result != tt.expected {
			t.Errorf("expandHome(%q, %q) = %q, want %q", tt.path, tt.home, result, tt.expected)
		}
	}
}

func TestInstallServicesNoEnvVar(t *testing.T) {
	os.Unsetenv("KD_SERVICES")
	err := installServices()
	if err != nil {
		t.Errorf("installServices() error = %v", err)
	}
}

func TestInstallServicesUnknownService(t *testing.T) {
	defer stubInstallCmd()()

	tmpDir := t.TempDir()
	orig := servicesMarkerPath
	servicesMarkerPath = filepath.Join(tmpDir, ".kilo-services-installed")
	defer func() { servicesMarkerPath = orig }()

	os.Setenv("KD_SERVICES", "nonexistent")
	defer os.Unsetenv("KD_SERVICES")

	err := installServices()
	if err != nil {
		t.Errorf("installServices() error = %v", err)
	}
}

func TestInstallServicesDockerService(t *testing.T) {
	defer stubInstallCmd()()

	tmpDir := t.TempDir()
	orig := servicesMarkerPath
	servicesMarkerPath = filepath.Join(tmpDir, ".kilo-services-installed")
	defer func() { servicesMarkerPath = orig }()

	os.Setenv("KD_SERVICES", "docker")
	defer os.Unsetenv("KD_SERVICES")

	err := installServices()
	if err != nil {
		t.Errorf("installServices() error = %v", err)
	}
}

func TestInstallServicesMultipleServices(t *testing.T) {
	defer stubInstallCmd()()

	tmpDir := t.TempDir()
	orig := servicesMarkerPath
	servicesMarkerPath = filepath.Join(tmpDir, ".kilo-services-installed")
	defer func() { servicesMarkerPath = orig }()

	os.Setenv("KD_SERVICES", "docker,zellij")
	defer os.Unsetenv("KD_SERVICES")

	err := installServices()
	if err != nil {
		t.Errorf("installServices() error = %v", err)
	}
}

func TestInstallServicesMarkerSkipsReinstall(t *testing.T) {
	defer stubInstallCmd()()

	tmpDir := t.TempDir()
	orig := servicesMarkerPath
	servicesMarkerPath = filepath.Join(tmpDir, ".kilo-services-installed")
	defer func() { servicesMarkerPath = orig }()

	os.Setenv("KD_SERVICES", "zellij")
	defer os.Unsetenv("KD_SERVICES")

	// First call should install and write marker.
	if err := installServices(); err != nil {
		t.Fatalf("first installServices() error = %v", err)
	}

	data, err := os.ReadFile(servicesMarkerPath)
	if err != nil {
		t.Fatalf("marker file not created: %v", err)
	}
	if strings.TrimSpace(string(data)) != "zellij" {
		t.Errorf("marker content = %q, want %q", strings.TrimSpace(string(data)), "zellij")
	}

	// Second call should detect the marker and skip installation.
	if err := installServices(); err != nil {
		t.Fatalf("second installServices() error = %v", err)
	}
}

func TestInstallServicesChangedServicesReinstalls(t *testing.T) {
	defer stubInstallCmd()()

	tmpDir := t.TempDir()
	orig := servicesMarkerPath
	servicesMarkerPath = filepath.Join(tmpDir, ".kilo-services-installed")
	defer func() { servicesMarkerPath = orig }()

	// Install with "zellij".
	os.Setenv("KD_SERVICES", "zellij")
	if err := installServices(); err != nil {
		t.Fatalf("first installServices() error = %v", err)
	}

	// Change to "zellij,gh" — should reinstall.
	os.Setenv("KD_SERVICES", "zellij,gh")
	defer os.Unsetenv("KD_SERVICES")

	if err := installServices(); err != nil {
		t.Fatalf("second installServices() error = %v", err)
	}

	data, err := os.ReadFile(servicesMarkerPath)
	if err != nil {
		t.Fatalf("marker file not readable: %v", err)
	}
	if strings.TrimSpace(string(data)) != "zellij,gh" {
		t.Errorf("marker content = %q, want %q", strings.TrimSpace(string(data)), "zellij,gh")
	}
}

func TestCopyServiceConfigsNoEnvVar(t *testing.T) {
	os.Unsetenv("KD_SERVICES")
	err := copyServiceConfigs("/home/test")
	if err != nil {
		t.Errorf("copyServiceConfigs() error = %v", err)
	}
}

func TestCopyServiceConfigsUnknownService(t *testing.T) {
	os.Setenv("KD_SERVICES", "nonexistent")
	defer os.Unsetenv("KD_SERVICES")

	err := copyServiceConfigs("/home/test")
	if err != nil {
		t.Errorf("copyServiceConfigs() error = %v", err)
	}
}

func TestCopyServiceConfigsZellijCreatesConfig(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "user")
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatal(err)
	}

	configDir := filepath.Join(homeDir, ".config", "zellij")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	srcFile := filepath.Join(configDir, "config.kdl")
	if err := os.WriteFile(srcFile, []byte("test config"), 0644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("KD_SERVICES", "zellij")
	defer os.Unsetenv("KD_SERVICES")

	err := copyServiceConfigs(homeDir)
	if err != nil {
		t.Errorf("copyServiceConfigs() error = %v", err)
	}

	dstFile := filepath.Join(homeDir, ".config", "zellij", "config.kdl")
	if _, err := os.Stat(dstFile); os.IsNotExist(err) {
		t.Error("expected config file to be copied")
	}
}

func TestCopyServiceConfigsSkipsExisting(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "user")
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatal(err)
	}

	configDir := filepath.Join(homeDir, ".config", "zellij")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	srcFile := filepath.Join(configDir, "config.kdl")
	if err := os.WriteFile(srcFile, []byte("source config"), 0644); err != nil {
		t.Fatal(err)
	}

	dstFile := filepath.Join(configDir, "config.kdl")
	if err := os.WriteFile(dstFile, []byte("existing config"), 0644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("KD_SERVICES", "zellij")
	defer os.Unsetenv("KD_SERVICES")

	err := copyServiceConfigs(homeDir)
	if err != nil {
		t.Errorf("copyServiceConfigs() error = %v", err)
	}

	data, _ := os.ReadFile(dstFile)
	if string(data) != "existing config" {
		t.Error("expected existing config to be preserved")
	}
}
