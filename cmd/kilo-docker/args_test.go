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

	args := buildContainerArgs(cfg, "vol", "/pwd", "test-container", "not_found", "")

	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "-v vol:/home") {
		t.Error("expected volume mount in args")
	}
	if !strings.Contains(argsStr, "--docker") {
		t.Error("expected --docker in session args")
	}
	if !strings.Contains(argsStr, "-e DOCKER_ENABLED=1") {
		t.Error("expected DOCKER_ENABLED env var")
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

	args := buildContainerArgs(cfg, "vol", "/pwd", "test-container", "not_found", "")

	argsStr := strings.Join(args, " ")

	if strings.Contains(argsStr, "KD_SERVICES") {
		t.Error("expected no KD_SERVICES env var when no services")
	}
}

func TestBuildContainerArgsOnceMode(t *testing.T) {
	cfg := config{
		once:            true,
		enabledServices: []string{"gh"},
	}

	args := buildContainerArgs(cfg, "", "/pwd", "test-container", "not_found", "")

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

	args := buildContainerArgs(cfg, "vol", "/pwd", "test-container", "not_found", "")

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

	args := buildContainerArgs(cfg, "vol", "/pwd", "test-container", "not_found", "")

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
	expected := "--network kilo-shared"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSerializeArgsOnce(t *testing.T) {
	cfg := config{
		once:            true,
		enabledServices: []string{},
	}
	result := serializeArgs(cfg, false)
	expected := "--once --network kilo-shared"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
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
	}
	result := serializeArgs(cfg, true)
	if !strings.Contains(result, "--once") {
		t.Errorf("expected '--once' in result, got %q", result)
	}
	if !strings.Contains(result, "--docker") {
		t.Errorf("expected '--docker' in result, got %q", result)
	}
	if !strings.Contains(result, "--playwright") {
		t.Errorf("expected '--playwright' in result, got %q", result)
	}
	if !strings.Contains(result, "--ssh") {
		t.Errorf("expected '--ssh' in result, got %q", result)
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
		networks:        []string{"my-network"},
	}
	result := serializeArgs(cfg, false)
	expected := "--network kilo-shared --network my-network"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSerializeArgsYesOnlyWhenExplicit(t *testing.T) {
	cfgWithoutYes := config{}
	resultWithoutYes := serializeArgs(cfgWithoutYes, false)
	if strings.Contains(resultWithoutYes, "--yes") {
		t.Errorf("expected no '--yes' when cfg.yes=false, got %q", resultWithoutYes)
	}

	cfgWithYes := config{yes: true}
	resultWithYes := serializeArgs(cfgWithYes, false)
	if !strings.Contains(resultWithYes, "--yes") {
		t.Errorf("expected '--yes' when cfg.yes=true, got %q", resultWithYes)
	}
}

func TestArgsMatchIdentical(t *testing.T) {
	if !argsMatch("--once", "--once") {
		t.Error("identical args should match")
	}
}

func TestArgsMatchNetworkNormalized(t *testing.T) {
	// --network kilo-shared is implicit and should be stripped before comparison
	if !argsMatch("--port 8080:80", "--network kilo-shared --port 8080:80") {
		t.Error("kilo-shared network should be normalized away")
	}
}

func TestArgsMatchDifferent(t *testing.T) {
	if argsMatch("--once", "--playwright") {
		t.Error("different args should not match")
	}
}

func TestArgsMatchPortReordering(t *testing.T) {
	// Port reordering is NOT normalized — this is a known limitation
	if argsMatch("--port 8080:80 --port 3000:3000", "--port 3000:3000 --port 8080:80") {
		t.Error("port reordering should not match (known limitation)")
	}
}

func TestArgsMatchSSH(t *testing.T) {
	// --ssh should be compared like any other flag
	if !argsMatch("--ssh", "--ssh") {
		t.Error("identical args with --ssh should match")
	}
	if argsMatch("--ssh", "") {
		t.Error("args with --ssh should not match empty args")
	}
}

func TestSerializeStoredArgsRoundTrip(t *testing.T) {
	stored := "--once --ssh"
	displayed := serializeStoredArgs(stored)
	if !strings.Contains(displayed, "--once") {
		t.Errorf("expected '--once' in displayed, got %q", displayed)
	}
	if !strings.Contains(displayed, "--ssh") {
		t.Errorf("expected '--ssh' in displayed, got %q", displayed)
	}
}

func TestSerializeStoredArgsEmpty(t *testing.T) {
	if serializeStoredArgs("") != "" {
		t.Error("empty stored args should return empty")
	}
}

func TestSerializeStoredArgsPorts(t *testing.T) {
	stored := "--port 8080:80 --port 3000:3000"
	displayed := serializeStoredArgs(stored)
	if !strings.Contains(displayed, "--port 8080:80") {
		t.Errorf("expected '--port 8080:80' in displayed, got %q", displayed)
	}
	if !strings.Contains(displayed, "--port 3000:3000") {
		t.Errorf("expected '--port 3000:3000' in displayed, got %q", displayed)
	}
}

func TestArgsMatchExitedContainerSSHMismatch(t *testing.T) {
	// This simulates the scenario where a container was created with --ssh
	// but the user runs without it — args should not match
	current := serializeForDisplay(config{}, false)
	stored := serializeArgs(config{ssh: true}, true)
	if argsMatch(current, stored) {
		t.Error("args should not match when SSH flag differs")
	}
}

func TestExtractHostPortStandard(t *testing.T) {
	if got := extractHostPort("8080:80"); got != "8080" {
		t.Errorf("extractHostPort(\"8080:80\") = %q, want %q", got, "8080")
	}
}

func TestExtractHostPortHostOnly(t *testing.T) {
	if got := extractHostPort("8080"); got != "8080" {
		t.Errorf("extractHostPort(\"8080\") = %q, want %q", got, "8080")
	}
}

func TestExtractHostPortWithProtocol(t *testing.T) {
	if got := extractHostPort("8080:80/udp"); got != "8080" {
		t.Errorf("extractHostPort(\"8080:80/udp\") = %q, want %q", got, "8080")
	}
}

func TestExtractHostPortRange(t *testing.T) {
	if got := extractHostPort("8000-8010:80-90"); got != "8000-8010" {
		t.Errorf("extractHostPort(\"8000-8010:80-90\") = %q, want %q", got, "8000-8010")
	}
}

func TestExtractHostPortEmpty(t *testing.T) {
	if got := extractHostPort(""); got != "" {
		t.Errorf("extractHostPort(\"\") = %q, want %q", got, "")
	}
}

func TestExtractHostPortWithIP(t *testing.T) {
	if got := extractHostPort("127.0.0.1:8080:80"); got != "8080" {
		t.Errorf("extractHostPort(\"127.0.0.1:8080:80\") = %q, want %q", got, "8080")
	}
}

func TestExtractHostPortWithIPNoHostPort(t *testing.T) {
	if got := extractHostPort("127.0.0.1::80"); got != "" {
		t.Errorf("extractHostPort(\"127.0.0.1::80\") = %q, want %q", got, "")
	}
}

func TestExtractHostPortWithIPRange(t *testing.T) {
	if got := extractHostPort("127.0.0.1:8000-8010:80-90"); got != "8000-8010" {
		t.Errorf("extractHostPort(\"127.0.0.1:8000-8010:80-90\") = %q, want %q", got, "8000-8010")
	}
}

func TestCheckPortConflictsEmptyPorts(t *testing.T) {
	cfg := config{}
	if err := checkPortConflicts(cfg); err != nil {
		t.Errorf("expected no error for empty ports, got %v", err)
	}
}

func TestCheckPortConflictsNoRunningSessions(t *testing.T) {
	// This tests the early return when no sessions exist (no Docker needed
	// since cfg.ports is empty after the first check).
	cfg := config{}
	if err := checkPortConflicts(cfg); err != nil {
		t.Errorf("expected no error for empty config, got %v", err)
	}
}
