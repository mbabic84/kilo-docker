package main

import (
	"strings"
	"testing"
)

func TestBuildContainerArgsWithDockerService(t *testing.T) {
	cfg := config{
		once:            false,
		enabledServices: []string{"docker"},
	}
	hostEnvVars := map[string]string{
		"DOCKER_GID": "1001",
	}

	args := buildContainerArgs(cfg, "vol", "/pwd", "test-container", "not_found",
		"", hostEnvVars, "", "", "", "", 0)

	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "-v vol:/home/kilo-t8x3m7kp") {
		t.Error("expected volume mount in args")
	}
	if !strings.Contains(argsStr, "--docker") {
		t.Error("expected --docker in session args")
	}
	if !strings.Contains(argsStr, "-e DOCKER_ENABLED=1") {
		t.Error("expected DOCKER_ENABLED env var")
	}
	if !strings.Contains(argsStr, "-e DOCKER_GID=1001") {
		t.Error("expected DOCKER_GID env var from hostEnvVars")
	}
	if !strings.Contains(argsStr, "-v /var/run/docker.sock:/var/run/docker.sock") {
		t.Error("expected docker socket volume")
	}
	if !strings.Contains(argsStr, "-e KD_SERVICES=docker") {
		t.Error("expected KD_SERVICES env var")
	}
}

func TestBuildContainerArgsNoServices(t *testing.T) {
	cfg := config{
		once:            false,
		enabledServices: []string{},
	}

	args := buildContainerArgs(cfg, "vol", "/pwd", "test-container", "not_found",
		"", nil, "", "", "", "", 0)

	argsStr := strings.Join(args, " ")

	if strings.Contains(argsStr, "KD_SERVICES") {
		t.Error("expected no KD_SERVICES env var when no services")
	}
}

func TestBuildContainerArgsHostEnvVarsOnlyWhenSet(t *testing.T) {
	cfg := config{
		once:            false,
		enabledServices: []string{"docker"},
	}

	args := buildContainerArgs(cfg, "vol", "/pwd", "test-container", "not_found",
		"", nil, "", "", "", "", 0)

	argsStr := strings.Join(args, " ")

	if strings.Contains(argsStr, "-e DOCKER_GID=") {
		t.Error("expected no DOCKER_GID when hostEnvVars doesn't have it")
	}
}

func TestBuildContainerArgsOnceMode(t *testing.T) {
	cfg := config{
		once:            true,
		enabledServices: []string{"gh"},
	}

	args := buildContainerArgs(cfg, "", "/pwd", "test-container", "not_found",
		"", nil, "", "", "", "", 0)

	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "--once") {
		t.Error("expected --once in session args")
	}
}

func TestBuildContainerArgsWithPorts(t *testing.T) {
	cfg := config{
		once:  false,
		ports: []string{"8080:80", "3000:3000"},
	}

	args := buildContainerArgs(cfg, "vol", "/pwd", "test-container", "not_found",
		"", nil, "", "", "", "", 0)

	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "-p 8080:80") {
		t.Error("expected -p 8080:80 in docker args")
	}
	if !strings.Contains(argsStr, "-p 3000:3000") {
		t.Error("expected -p 3000:3000 in docker args")
	}
	if !strings.Contains(argsStr, "--port 8080:80") {
		t.Error("expected --port 8080:80 in session args label")
	}
	if !strings.Contains(argsStr, "--port 3000:3000") {
		t.Error("expected --port 3000:3000 in session args label")
	}
}

func TestBuildContainerArgsNoPorts(t *testing.T) {
	cfg := config{
		once: false,
	}

	args := buildContainerArgs(cfg, "vol", "/pwd", "test-container", "not_found",
		"", nil, "", "", "", "", 0)

	argsStr := strings.Join(args, " ")

	if strings.Contains(argsStr, "-p ") {
		t.Error("expected no -p flag when no ports configured")
	}
	if strings.Contains(argsStr, "--port") {
		t.Error("expected no --port in session args when no ports configured")
	}
}

func TestSerializeArgsEmpty(t *testing.T) {
	cfg := config{
		once:            false,
		enabledServices: []string{},
	}
	result := serializeArgs(cfg, false)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestSerializeArgsOnce(t *testing.T) {
	cfg := config{
		once:            true,
		enabledServices: []string{},
	}
	result := serializeArgs(cfg, false)
	if result != "--once" {
		t.Errorf("expected '--once', got %q", result)
	}
}

func TestSerializeArgsServices(t *testing.T) {
	cfg := config{
		once:            false,
		enabledServices: []string{"docker", "gh"},
	}
	result := serializeArgs(cfg, false)
	if !strings.Contains(result, "--docker") {
		t.Errorf("expected '--docker' in result, got %q", result)
	}
	if !strings.Contains(result, "--gh") {
		t.Errorf("expected '--gh' in result, got %q", result)
	}
}

func TestSerializeArgsCombined(t *testing.T) {
	cfg := config{
		once:            true,
		enabledServices: []string{"docker"},
		playwright:      true,
		mcp:             true,
	}
	result := serializeArgs(cfg, true)
	expected := "--once --docker --playwright --ssh --mcp"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSerializeArgsPorts(t *testing.T) {
	cfg := config{
		once:            false,
		enabledServices: []string{},
		ports:           []string{"8080:80", "3000:3000"},
	}
	result := serializeArgs(cfg, false)
	if !strings.Contains(result, "--port 8080:80") {
		t.Errorf("expected '--port 8080:80' in result, got %q", result)
	}
	if !strings.Contains(result, "--port 3000:3000") {
		t.Errorf("expected '--port 3000:3000' in result, got %q", result)
	}
}

func TestSerializeArgsNetwork(t *testing.T) {
	cfg := config{
		once:            false,
		enabledServices: []string{},
		network:         "my-network",
	}
	result := serializeArgs(cfg, false)
	expected := "--network my-network"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
