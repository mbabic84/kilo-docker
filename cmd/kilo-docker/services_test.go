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
	if svc.Flag != "--docker" {
		t.Errorf("expected flag '--docker', got %q", svc.Flag)
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
		enabledServices: []string{"docker", "gh"},
	}

	if !isServiceEnabled(cfg, "docker") {
		t.Error("expected docker to be enabled")
	}
	if !isServiceEnabled(cfg, "gh") {
		t.Error("expected gh to be enabled")
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
	if len(services.BuiltInServices) != 8 {
		t.Errorf("expected 8 built-in services, got %d", len(services.BuiltInServices))
	}
}

func TestDockerServiceHasRequiredFields(t *testing.T) {
	svc := mustGetService(t, "docker")

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

func TestGhServiceHasRequiredFields(t *testing.T) {
	svc := mustGetService(t, "gh")

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
	svc := mustGetService(t, "go")

	if svc.Name != "go" {
		t.Errorf("expected Name 'go', got %q", svc.Name)
	}
	if svc.Flag != "--go" {
		t.Errorf("expected Flag '--go', got %q", svc.Flag)
	}
	if len(svc.Install) != 5 {
		t.Errorf("expected 5 Install commands for go, got %d", len(svc.Install))
	}
	if svc.RequiresSocket != "" {
		t.Errorf("expected RequiresSocket to be empty for go, got %q", svc.RequiresSocket)
	}
}

func TestNvmServiceHasRequiredFields(t *testing.T) {
	svc := mustGetService(t, "nvm")

	if svc.Name != "nvm" {
		t.Errorf("expected Name 'nvm', got %q", svc.Name)
	}
	if svc.Flag != "--nvm" {
		t.Errorf("expected Flag '--nvm', got %q", svc.Flag)
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

func TestBuildServiceHasRequiredFields(t *testing.T) {
	svc := mustGetService(t, "build")

	if svc.Name != "build" {
		t.Errorf("expected Name 'build', got %q", svc.Name)
	}
	if svc.Flag != "--build" {
		t.Errorf("expected Flag '--build', got %q", svc.Flag)
	}
	if len(svc.Install) != 2 {
		t.Errorf("expected 2 Install commands for build, got %d", len(svc.Install))
	}
	if svc.RequiresSocket != "" {
		t.Errorf("expected RequiresSocket to be empty for build, got %q", svc.RequiresSocket)
	}
	if _, ok := svc.EnvVars["BUILD_ENABLED"]; !ok {
		t.Error("expected BUILD_ENABLED in EnvVars")
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

func TestGitNexusServiceHasRequiredFields(t *testing.T) {
	svc := mustGetService(t, "gitnexus")

	if svc.Name != "gitnexus" {
		t.Errorf("expected Name 'gitnexus', got %q", svc.Name)
	}
	if svc.Flag != "--gitnexus" {
		t.Errorf("expected Flag '--gitnexus', got %q", svc.Flag)
	}
	if len(svc.Install) != 0 {
		t.Errorf("expected 0 Install commands for gitnexus, got %d", len(svc.Install))
	}
	if len(svc.UserInstall) != 1 {
		t.Errorf("expected 1 UserInstall command for gitnexus, got %d", len(svc.UserInstall))
	}
	if svc.RequiresSocket != "" {
		t.Errorf("expected RequiresSocket to be empty for gitnexus, got %q", svc.RequiresSocket)
	}
	if _, ok := svc.EnvVars["GITNEXUS_ENABLED"]; !ok {
		t.Error("expected GITNEXUS_ENABLED in EnvVars")
	}
}
