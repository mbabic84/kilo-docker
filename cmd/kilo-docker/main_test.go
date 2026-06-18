package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestHelpProfileSubcommand verifies that 'kilo-docker help profile save'
// routes to the profile save help page, matching the behavior for
// 'sessions' and 'update' nested commands.
func TestHelpProfileSubcommand(t *testing.T) {
	bin := findKiloDockerBinary(t)

	cmd := exec.Command(bin, "help", "profile", "save")
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err != nil {
		t.Fatalf("expected help profile save to exit cleanly, got err=%v\noutput: %s", err, output)
	}
	if !strings.Contains(output, "Usage: kilo-docker profile save") {
		t.Errorf("expected profile save help, got:\n%s", output)
	}
}

// TestHelpProfileNoSubcommand verifies that 'kilo-docker help profile' shows
// the profile command help page, not the general help page.
func TestHelpProfileNoSubcommand(t *testing.T) {
	bin := findKiloDockerBinary(t)

	cmd := exec.Command(bin, "help", "profile")
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err != nil {
		t.Fatalf("expected help profile to exit cleanly, got err=%v\noutput: %s", err, output)
	}
	if !strings.Contains(output, "Usage: kilo-docker profile <command>") {
		t.Errorf("expected profile command help, got:\n%s", output)
	}
	if !strings.Contains(output, "save <name> <flags>") {
		t.Errorf("expected profile subcommand list, got:\n%s", output)
	}
}

// findKiloDockerBinary locates the pre-built kilo-docker binary.
func findKiloDockerBinary(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	testDir := filepath.Dir(filename)

	relPath := filepath.Join(testDir, "..", "..", "bin", "kilo-docker")
	abs, err := filepath.Abs(relPath)
	if err == nil {
		if _, statErr := filepath.Glob(abs); statErr == nil {
			if _, lookErr := exec.LookPath(abs); lookErr == nil {
				return abs
			}
			cmd := exec.Command(abs)
			cmd.Stdin = nil
			if runErr := cmd.Run(); runErr == nil {
				return abs
			}
		}
	}

	candidates := []string{
		relPath,
		"../../bin/kilo-docker",
		"/build/bin/kilo-docker",
	}
	for _, c := range candidates {
		absPath, _ := filepath.Abs(c)
		if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
			return absPath
		}
	}

	if path, err := exec.LookPath("kilo-docker"); err == nil {
		return path
	}

	t.Skip("kilo-docker binary not found; run './scripts/build.sh build-kilo-docker' first or skip integration tests")
	return ""
}
