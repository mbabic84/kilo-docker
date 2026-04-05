package main

import (
	"os"
	"testing"
)

// TestParseFlagsYesLong tests that --yes sets cfg.yes = true.
func TestParseFlagsYesLong(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"kilo-docker", "--yes", "install"}
	cfg := parseFlags()

	if !cfg.yes {
		t.Error("expected cfg.yes = true for --yes flag")
	}
	if cfg.command != "install" {
		t.Errorf("expected command = %q, got %q", "install", cfg.command)
	}
}

// TestParseFlagsYesShort tests that -y sets cfg.yes = true.
func TestParseFlagsYesShort(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"kilo-docker", "-y", "install"}
	cfg := parseFlags()

	if !cfg.yes {
		t.Error("expected cfg.yes = true for -y flag")
	}
	if cfg.command != "install" {
		t.Errorf("expected command = %q, got %q", "install", cfg.command)
	}
}

// TestParseFlagsYesNotSet tests that cfg.yes is false when the flag is absent.
func TestParseFlagsYesNotSet(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"kilo-docker", "install"}
	cfg := parseFlags()

	if cfg.yes {
		t.Error("expected cfg.yes = false without --yes/-y flag")
	}
}

// TestParseFlagsYesAfterCommand tests that -y works when placed after the command.
func TestParseFlagsYesAfterCommand(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"kilo-docker", "install", "-y"}
	cfg := parseFlags()

	if !cfg.yes {
		t.Error("expected cfg.yes = true for -y flag after command")
	}
	if cfg.command != "install" {
		t.Errorf("expected command = %q, got %q", "install", cfg.command)
	}
}

// TestParseFlagsYesWithOtherFlags tests that -y combines with other flags.
func TestParseFlagsYesWithOtherFlags(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"kilo-docker", "-y", "--docker", "install"}
	cfg := parseFlags()

	if !cfg.yes {
		t.Error("expected cfg.yes = true")
	}
	if !isServiceEnabled(cfg, "docker") {
		t.Error("expected docker service to be enabled")
	}
	if cfg.command != "install" {
		t.Errorf("expected command = %q, got %q", "install", cfg.command)
	}
}

// TestParseFlagsYesWithoutCommand tests that -y works with no command.
func TestParseFlagsYesWithoutCommand(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"kilo-docker", "-y"}
	cfg := parseFlags()

	if !cfg.yes {
		t.Error("expected cfg.yes = true for standalone -y flag")
	}
	if cfg.command != "" {
		t.Errorf("expected empty command, got %q", cfg.command)
	}
}

// TestPromptConfirmYesFlag tests that promptConfirm returns true
// when yes=true is passed, without reading from stdin.
func TestPromptConfirmYesFlag(t *testing.T) {
	result := promptConfirm("Continue? [y/N]: ", true)
	if !result {
		t.Error("expected promptConfirm to return true when yes=true")
	}
}

// TestPromptConfirmReadsStdin tests that promptConfirm reads from stdin
// when yes=false, returning true for "y" input.
func TestPromptConfirmReadsStdin(t *testing.T) {
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdin = r

	go func() {
		w.WriteString("y\n")
		w.Close()
	}()

	result := promptConfirm("Continue? [y/N]: ", false)
	if !result {
		t.Error("expected promptConfirm to return true for 'y' input")
	}
}

// TestPromptConfirmRejectsEmpty tests that promptConfirm returns false
// for empty input when yes=false.
func TestPromptConfirmRejectsEmpty(t *testing.T) {
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdin = r

	go func() {
		w.WriteString("\n")
		w.Close()
	}()

	result := promptConfirm("Continue? [y/N]: ", false)
	if result {
		t.Error("expected promptConfirm to return false for empty input")
	}
}

// TestPromptConfirmRejectsN tests that promptConfirm returns false
// for "n" input when yes=false.
func TestPromptConfirmRejectsN(t *testing.T) {
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdin = r

	go func() {
		w.WriteString("n\n")
		w.Close()
	}()

	result := promptConfirm("Continue? [y/N]: ", false)
	if result {
		t.Error("expected promptConfirm to return false for 'n' input")
	}
}

// TestIsTerminalWithPipedStdin verifies that isTerminal() returns false
// when stdin is a pipe.
func TestIsTerminalWithPipedStdin(t *testing.T) {
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdin = r

	// Close the write end so stdin reads EOF (simulates piped input ending)
	w.Close()

	// With piped stdin, isTerminal() should return false
	if isTerminal() {
		t.Error("expected isTerminal() = false with piped stdin")
	}
}

// TestYesFlagDirectlyPassed verifies that -y flag is correctly parsed
// and passed to prompt functions.
func TestYesFlagDirectlyPassed(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"kilo-docker", "-y", "cleanup"}
	cfg := parseFlags()

	if !cfg.yes {
		t.Error("expected cfg.yes = true with -y flag")
	}
}

// TestParseFlagsPortLong tests that --port adds a port mapping.
func TestParseFlagsPortLong(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"kilo-docker", "--port", "8080:80"}
	cfg := parseFlags()

	if len(cfg.ports) != 1 || cfg.ports[0] != "8080:80" {
		t.Errorf("expected ports = [8080:80], got %v", cfg.ports)
	}
}

// TestParseFlagsPortShort tests that -p adds a port mapping.
func TestParseFlagsPortShort(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"kilo-docker", "-p", "3000:3000"}
	cfg := parseFlags()

	if len(cfg.ports) != 1 || cfg.ports[0] != "3000:3000" {
		t.Errorf("expected ports = [3000:3000], got %v", cfg.ports)
	}
}

// TestParseFlagsPortMultiple tests that multiple --port/-p flags are accumulated.
func TestParseFlagsPortMultiple(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"kilo-docker", "-p", "8080:80", "--port", "3000:3000", "-p", "443:443"}
	cfg := parseFlags()

	if len(cfg.ports) != 3 {
		t.Fatalf("expected 3 ports, got %d: %v", len(cfg.ports), cfg.ports)
	}
	if cfg.ports[0] != "8080:80" || cfg.ports[1] != "3000:3000" || cfg.ports[2] != "443:443" {
		t.Errorf("expected ports = [8080:80 3000:3000 443:443], got %v", cfg.ports)
	}
}

// TestParseFlagsPortNotSet tests that ports is nil when no --port flag is given.
func TestParseFlagsPortNotSet(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"kilo-docker"}
	cfg := parseFlags()

	if len(cfg.ports) != 0 {
		t.Errorf("expected no ports, got %v", cfg.ports)
	}
}

// TestParseFlagsPortWithOtherFlags tests that --port combines with other flags.
func TestParseFlagsPortWithOtherFlags(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	os.Args = []string{"kilo-docker", "-p", "8080:80", "--docker", "install"}
	cfg := parseFlags()

	if len(cfg.ports) != 1 || cfg.ports[0] != "8080:80" {
		t.Errorf("expected ports = [8080:80], got %v", cfg.ports)
	}
	if !isServiceEnabled(cfg, "docker") {
		t.Error("expected docker service to be enabled")
	}
	if cfg.command != "install" {
		t.Errorf("expected command = %q, got %q", "install", cfg.command)
	}
}
