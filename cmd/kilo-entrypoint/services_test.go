package main

import (
	"testing"

	"github.com/mbabic84/kilo-docker/pkg/services"
)

func mustGetService(t *testing.T, name string) *services.Service {
	t.Helper()
	svc := getService(name)
	if svc == nil {
		t.Fatalf("service %q not found", name)
	}
	return svc
}

func TestGetServiceDocker(t *testing.T) {
	svc := mustGetService(t, "docker")
	if svc.Name != "docker" {
		t.Errorf("expected name 'docker', got %q", svc.Name)
	}
}

func TestGetServiceUnknown(t *testing.T) {
	svc := getService("nonexistent")
	if svc != nil {
		t.Errorf("expected nil for unknown service, got %+v", svc)
	}
}

func TestBuiltInServicesCount(t *testing.T) {
	if len(services.BuiltInServices) != 8 {
		t.Errorf("expected 8 built-in services, got %d", len(services.BuiltInServices))
	}
}

func TestGhServiceHasInstallCommands(t *testing.T) {
	svc := mustGetService(t, "gh")

	if len(svc.Install) == 0 {
		t.Error("expected Install to have commands")
	}
}

func TestGhServiceHasSystemVersionCheck(t *testing.T) {
	svc := mustGetService(t, "gh")

	if svc.InstallVersionCheck == "" {
		t.Error("expected InstallVersionCheck to be set for gh")
	}
	if svc.InstallLatestVersion == "" {
		t.Error("expected InstallLatestVersion to be set for gh")
	}
}

func TestGhServiceDisplayName(t *testing.T) {
	svc := mustGetService(t, "gh")

	if svc.DisplayName != "gh-mcp extension" {
		t.Errorf("expected DisplayName 'gh-mcp extension', got %q", svc.DisplayName)
	}
	if got := svc.DisplayNameOrName(); got != "gh-mcp extension" {
		t.Errorf("expected DisplayNameOrName() 'gh-mcp extension', got %q", got)
	}
}

func TestDisplayNameOrNameFallsBackToName(t *testing.T) {
	svc := mustGetService(t, "docker")

	if got := svc.DisplayNameOrName(); got != "docker" {
		t.Errorf("expected DisplayNameOrName() to fall back to Name 'docker', got %q", got)
	}
}

func TestDockerServiceHasInstallCommands(t *testing.T) {
	svc := mustGetService(t, "docker")

	if len(svc.Install) == 0 {
		t.Error("expected Install to have commands")
	}
}

func TestGoServiceHasInstallCommands(t *testing.T) {
	svc := mustGetService(t, "go")

	if len(svc.Install) != 5 {
		t.Errorf("expected 5 Install commands for go, got %d", len(svc.Install))
	}
}

func TestNvmServiceHasRequiredFields(t *testing.T) {
	svc := mustGetService(t, "nvm")

	if svc.Name != "nvm" {
		t.Errorf("expected Name 'nvm', got %q", svc.Name)
	}
	if len(svc.UserInstall) != 1 {
		t.Errorf("expected 1 UserInstall command for nvm, got %d", len(svc.UserInstall))
	}
	if len(svc.Install) != 0 {
		t.Errorf("expected 0 Install commands for nvm, got %d", len(svc.Install))
	}
	if svc.RequiresSocket != "" {
		t.Errorf("expected RequiresSocket to be empty for nvm, got %q", svc.RequiresSocket)
	}
}

func TestRcloneServiceHasRequiredFields(t *testing.T) {
	svc := mustGetService(t, "rclone")

	if svc.Name != "rclone" {
		t.Errorf("expected Name 'rclone', got %q", svc.Name)
	}
	if svc.Flag != "--rclone" {
		t.Errorf("expected Flag '--rclone', got %q", svc.Flag)
	}
	if len(svc.Install) != 1 {
		t.Errorf("expected 1 Install command for rclone, got %d", len(svc.Install))
	}
	if svc.RequiresSocket != "" {
		t.Errorf("expected RequiresSocket to be empty for rclone, got %q", svc.RequiresSocket)
	}
}
