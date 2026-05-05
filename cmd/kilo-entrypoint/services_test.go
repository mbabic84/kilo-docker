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

func TestGetServiceUnknown(t *testing.T) {
	svc := getService("nonexistent")
	if svc != nil {
		t.Errorf("expected nil for unknown service, got %+v", svc)
	}
}

func TestBuiltInServicesCount(t *testing.T) {
	if len(services.BuiltInServices) != 9 {
		t.Errorf("expected 9 built-in services, got %d", len(services.BuiltInServices))
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

func TestGoServiceHasInstallCommands(t *testing.T) {
	svc := getService("go")
	if svc == nil {
		t.Fatal("go service not found")
	}

	if len(svc.Install) != 5 {
		t.Errorf("expected 5 Install commands for go, got %d", len(svc.Install))
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

func TestS5cmdServiceHasRequiredFields(t *testing.T) {
	svc := getService("s5cmd")
	if svc == nil {
		t.Fatal("s5cmd service not found")
	}

	if svc.Name != "s5cmd" {
		t.Errorf("expected Name 's5cmd', got %q", svc.Name)
	}
	if svc.Flag != "--s5cmd" {
		t.Errorf("expected Flag '--s5cmd', got %q", svc.Flag)
	}
	if len(svc.Install) != 1 {
		t.Errorf("expected 1 Install command for s5cmd, got %d", len(svc.Install))
	}
	if svc.RequiresSocket != "" {
		t.Errorf("expected RequiresSocket to be empty for s5cmd, got %q", svc.RequiresSocket)
	}
}
