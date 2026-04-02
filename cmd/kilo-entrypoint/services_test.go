package main

import (
	"testing"

	"github.com/mbabic84/kilo-docker/pkg/services"
)

func TestGetServiceDocker(t *testing.T) {
	svc := getService("docker")
	if svc == nil {
		t.Fatal("expected docker service, got nil")
	}
	if svc.Name != "docker" {
		t.Errorf("expected name 'docker', got %q", svc.Name)
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
	svc := getService("gh")
	if svc == nil {
		t.Fatal("gh service not found")
	}

	if len(svc.Install) == 0 {
		t.Error("expected Install to have commands")
	}
}

func TestDockerServiceHasInstallCommands(t *testing.T) {
	svc := getService("docker")
	if svc == nil {
		t.Fatal("docker service not found")
	}

	if len(svc.Install) == 0 {
		t.Error("expected Install to have commands")
	}
}

func TestZellijServiceHasInstallCommands(t *testing.T) {
	svc := getService("zellij")
	if svc == nil {
		t.Fatal("zellij service not found")
	}

	if len(svc.Install) == 0 {
		t.Error("expected Install to have commands")
	}
}

func TestZellijServiceHasCopyConfigs(t *testing.T) {
	svc := getService("zellij")
	if svc == nil {
		t.Fatal("zellij service not found")
	}

	if len(svc.CopyConfigs) == 0 {
		t.Error("expected CopyConfigs to be set for zellij")
	}
	if len(svc.CopyConfigs) > 0 {
		cfg := svc.CopyConfigs[0]
		if cfg.Src == "" {
			t.Error("expected CopyConfig Src to be set")
		}
		if cfg.Dst == "" {
			t.Error("expected CopyConfig Dst to be set")
		}
	}
}

func TestGoServiceHasInstallCommands(t *testing.T) {
	svc := getService("go")
	if svc == nil {
		t.Fatal("go service not found")
	}

	if len(svc.Install) != 4 {
		t.Errorf("expected 4 Install commands for go, got %d", len(svc.Install))
	}
}

func TestNvmServiceHasRequiredFields(t *testing.T) {
	svc := getService("nvm")
	if svc == nil {
		t.Fatal("nvm service not found")
	}

	if svc.Name != "nvm" {
		t.Errorf("expected Name 'nvm', got %q", svc.Name)
	}
	if len(svc.Install) != 3 {
		t.Errorf("expected 3 Install commands for nvm, got %d", len(svc.Install))
	}
	if svc.RequiresSocket != "" {
		t.Errorf("expected RequiresSocket to be empty for nvm, got %q", svc.RequiresSocket)
	}
}

func TestPythonServiceHasRequiredFields(t *testing.T) {
	svc := getService("python")
	if svc == nil {
		t.Fatal("python service not found")
	}

	if svc.Name != "python" {
		t.Errorf("expected Name 'python', got %q", svc.Name)
	}
	if len(svc.Install) != 2 {
		t.Errorf("expected 2 Install commands for python, got %d", len(svc.Install))
	}
	if svc.RequiresSocket != "" {
		t.Errorf("expected RequiresSocket to be empty for python, got %q", svc.RequiresSocket)
	}
}
