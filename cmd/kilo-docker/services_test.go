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
	if len(services.BuiltInServices) != 9 {
		t.Errorf("expected 9 built-in services, got %d", len(services.BuiltInServices))
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
	if len(svc.Install) != 5 {
		t.Errorf("expected 5 Install commands for go, got %d", len(svc.Install))
	}
	if svc.RequiresSocket != "" {
		t.Errorf("expected RequiresSocket to be empty for go, got %q", svc.RequiresSocket)
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
	if _, ok := svc.EnvVars["NVM_NODEJS_ORG_MIRROR"]; !ok {
		t.Error("expected NVM_NODEJS_ORG_MIRROR in EnvVars")
	}
}

func TestBuildServiceHasRequiredFields(t *testing.T) {
	svc := getService("build")
	if svc == nil {
		t.Fatal("build service not found")
	}

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

func TestPythonServiceHasRequiredFields(t *testing.T) {
	svc := getService("python")
	if svc == nil {
		t.Fatal("python service not found")
	}

	if svc.Name != "python" {
		t.Errorf("expected Name 'python', got %q", svc.Name)
	}
	if svc.Flag != "--python" {
		t.Errorf("expected Flag '--python', got %q", svc.Flag)
	}
	if len(svc.Install) != 2 {
		t.Errorf("expected 2 Install commands for python, got %d", len(svc.Install))
	}
	if svc.RequiresSocket != "" {
		t.Errorf("expected RequiresSocket to be empty for python, got %q", svc.RequiresSocket)
	}
	if _, ok := svc.EnvVars["PYTHON_ENABLED"]; !ok {
		t.Error("expected PYTHON_ENABLED in EnvVars")
	}
}

func TestRcloneServiceHasRequiredFields(t *testing.T) {
	svc := getService("rclone")
	if svc == nil {
		t.Fatal("rclone service not found")
	}

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
	svc := getService("gitnexus")
	if svc == nil {
		t.Fatal("gitnexus service not found")
	}

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
