package main

import (
	"testing"

	"github.com/kilo-org/kilo-docker/pkg/services"
)

func TestGetServiceDocker(t *testing.T) {
	svc := getService("docker")
	if svc == nil {
		t.Fatal("expected docker service, got nil")
	}
	if svc.Name != "docker" {
		t.Errorf("expected name 'docker', got %q", svc.Name)
	}
	if svc.Flag != "--docker" {
		t.Errorf("expected flag '--docker', got %q", svc.Flag)
	}
}

func TestGetServiceZellij(t *testing.T) {
	svc := getService("zellij")
	if svc == nil {
		t.Fatal("expected zellij service, got nil")
	}
	if svc.Name != "zellij" {
		t.Errorf("expected name 'zellij', got %q", svc.Name)
	}
	if svc.Flag != "--zellij" {
		t.Errorf("expected flag '--zellij', got %q", svc.Flag)
	}
}

func TestGetServiceUnknown(t *testing.T) {
	svc := getService("nonexistent")
	if svc != nil {
		t.Errorf("expected nil for unknown service, got %+v", svc)
	}
}

func TestIsServiceEnabled(t *testing.T) {
	cfg := config{
		enabledServices: []string{"docker", "zellij"},
	}

	if !isServiceEnabled(cfg, "docker") {
		t.Error("expected docker to be enabled")
	}
	if !isServiceEnabled(cfg, "zellij") {
		t.Error("expected zellij to be enabled")
	}
	if isServiceEnabled(cfg, "nonexistent") {
		t.Error("expected nonexistent to not be enabled")
	}
}

func TestIsServiceEnabledEmpty(t *testing.T) {
	cfg := config{}

	if isServiceEnabled(cfg, "docker") {
		t.Error("expected docker to not be enabled when no services")
	}
}

func TestBuiltInServicesCount(t *testing.T) {
	if len(services.BuiltInServices) != 6 {
		t.Errorf("expected 6 built-in services, got %d", len(services.BuiltInServices))
	}
}

func TestDockerServiceHasRequiredFields(t *testing.T) {
	svc := getService("docker")
	if svc == nil {
		t.Fatal("docker service not found")
	}

	if svc.Name != "docker" {
		t.Errorf("expected Name 'docker', got %q", svc.Name)
	}
	if svc.Flag != "--docker" {
		t.Errorf("expected Flag '--docker', got %q", svc.Flag)
	}
	if len(svc.Install) == 0 {
		t.Error("expected Install to have commands")
	}
	if svc.Volumes == nil {
		t.Error("expected Volumes to be set")
	}
	if svc.RequiresSocket == "" {
		t.Error("expected RequiresSocket to be set for docker")
	}
	if _, ok := svc.EnvVars["DOCKER_ENABLED"]; !ok {
		t.Error("expected DOCKER_ENABLED in EnvVars")
	}
}

func TestZellijServiceHasRequiredFields(t *testing.T) {
	svc := getService("zellij")
	if svc == nil {
		t.Fatal("zellij service not found")
	}

	if svc.Name != "zellij" {
		t.Errorf("expected Name 'zellij', got %q", svc.Name)
	}
	if svc.Flag != "--zellij" {
		t.Errorf("expected Flag '--zellij', got %q", svc.Flag)
	}
	if len(svc.Install) == 0 {
		t.Error("expected Install to have commands")
	}
	if svc.RequiresSocket != "" {
		t.Errorf("expected RequiresSocket to be empty for zellij, got %q", svc.RequiresSocket)
	}
	if _, ok := svc.EnvVars["ZELLIJ_ENABLED"]; !ok {
		t.Error("expected ZELLIJ_ENABLED in EnvVars")
	}
	if len(svc.CopyConfigs) == 0 {
		t.Error("expected CopyConfigs to be set for zellij")
	}
}

func TestGhServiceHasRequiredFields(t *testing.T) {
	svc := getService("gh")
	if svc == nil {
		t.Fatal("gh service not found")
	}

	if svc.Name != "gh" {
		t.Errorf("expected Name 'gh', got %q", svc.Name)
	}
	if svc.Flag != "--gh" {
		t.Errorf("expected Flag '--gh', got %q", svc.Flag)
	}
	if len(svc.Install) == 0 {
		t.Error("expected Install to have commands")
	}
	if svc.RequiresSocket != "" {
		t.Errorf("expected RequiresSocket to be empty for gh, got %q", svc.RequiresSocket)
	}
}

func TestGoServiceHasRequiredFields(t *testing.T) {
	svc := getService("go")
	if svc == nil {
		t.Fatal("go service not found")
	}

	if svc.Name != "go" {
		t.Errorf("expected Name 'go', got %q", svc.Name)
	}
	if svc.Flag != "--go" {
		t.Errorf("expected Flag '--go', got %q", svc.Flag)
	}
	if len(svc.Install) != 4 {
		t.Errorf("expected 4 Install commands for go, got %d", len(svc.Install))
	}
	if svc.RequiresSocket != "" {
		t.Errorf("expected RequiresSocket to be empty for go, got %q", svc.RequiresSocket)
	}
}
