package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- Unit tests for subcommands map ---

// TestSubcommandsContainsAllInternal verifies that all documented
// internal subcommands are registered in the subcommands map.
func TestSubcommandsContainsAllInternal(t *testing.T) {
	expected := []string{
		"load-tokens",
		"save-tokens",
		"ainstruct-login",
		"update-config",
		"backup",
		"restore",
		"config",
		"sync",
		"resync",
	}

	for _, name := range expected {
		if !subcommands[name] {
			t.Errorf("subcommand %q is missing from subcommands map", name)
		}
	}

	if len(subcommands) != len(expected) {
		extra := []string{}
		for k := range subcommands {
			found := false
			for _, e := range expected {
				if k == e {
					found = true
					break
				}
			}
			if !found {
				extra = append(extra, k)
			}
		}
		t.Errorf("subcommands map contains unexpected entries: %v", extra)
	}
}

// TestSubcommandsExcludesPassThroughCommands verifies that common
// binaries used as pass-through commands are NOT in the subcommands map,
// ensuring they fall through to exec.LookPath.
func TestSubcommandsExcludesPassThroughCommands(t *testing.T) {
	passThroughCommands := []string{
		"kilo",
		"sh",
		"bash",
		"ls",
		"cat",
		"echo",
		"git",
		"node",
		"python",
	}

	for _, name := range passThroughCommands {
		if subcommands[name] {
			t.Errorf("pass-through command %q should NOT be in subcommands map", name)
		}
	}
}

// --- Unit tests for resolveCommand ---

// TestResolveCommandInternalSubcommands verifies that resolveCommand
// returns ("", false) for all internal subcommands, meaning they are
// NOT passed through to exec.
func TestResolveCommandInternalSubcommands(t *testing.T) {
	internal := []string{
		"load-tokens",
		"save-tokens",
		"ainstruct-login",
		"update-config",
		"backup",
		"restore",
		"config",
		"sync",
		"resync",
	}

	for _, name := range internal {
		binary, passthrough := resolveCommand(name)
		if passthrough {
			t.Errorf("resolveCommand(%q): expected internal (passthrough=false), got passthrough=true with binary=%q", name, binary)
		}
		if binary != "" {
			t.Errorf("resolveCommand(%q): expected empty binary for internal subcommand, got %q", name, binary)
		}
	}
}

// TestResolveCommandPassThroughKnownBinary verifies that resolveCommand
// returns a non-empty binary path and passthrough=true for known system
// binaries that exist on the test PATH.
func TestResolveCommandPassThroughKnownBinary(t *testing.T) {
	// "sh" should exist on any Unix system
	binary, passthrough := resolveCommand("sh")
	if !passthrough {
		t.Fatal("resolveCommand(\"sh\"): expected passthrough=true")
	}
	if binary == "" {
		t.Fatal("resolveCommand(\"sh\"): expected non-empty binary path")
	}
	if !strings.Contains(binary, "sh") {
		t.Errorf("resolveCommand(\"sh\"): binary path %q doesn't contain 'sh'", binary)
	}
}

// TestResolveCommandPassThroughKilo verifies that "kilo" is treated
// as a pass-through command (the original bug: it was hitting the
// "unknown subcommand" error instead of exec).
func TestResolveCommandPassThroughKilo(t *testing.T) {
	binary, passthrough := resolveCommand("kilo")
	if !passthrough {
		t.Fatal("resolveCommand(\"kilo\"): expected passthrough=true, got false — this was the original bug")
	}
	// binary may be empty if kilo is not on PATH, which is fine —
	// the important thing is passthrough=true so main() tries to exec it.
	if binary == "" {
		t.Log("kilo binary not found on PATH (expected in test env), but passthrough=true is correct")
	}
}

// TestResolveCommandUnknownBinary verifies that resolveCommand returns
// passthrough=true with empty binary for completely unknown commands.
func TestResolveCommandUnknownBinary(t *testing.T) {
	binary, passthrough := resolveCommand("nonexistent-binary-xyz-999")
	if !passthrough {
		t.Fatal("resolveCommand(\"nonexistent-binary-xyz-999\"): expected passthrough=true")
	}
	if binary != "" {
		t.Errorf("resolveCommand(\"nonexistent-binary-xyz-999\"): expected empty binary, got %q", binary)
	}
}

// --- Integration tests (run compiled entrypoint as subprocess) ---

// TestPassThroughExecSh verifies that invoking the entrypoint with
// "sh" as the first argument results in exec of /bin/sh.
func TestPassThroughExecSh(t *testing.T) {
	bin := findEntrypointBinary(t)

	cmd := exec.Command(bin, "sh", "-c", "echo pass-through-works")
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	if err != nil {
		t.Fatalf("pass-through to 'sh' failed: %v\noutput: %s", err, output)
	}

	if output != "pass-through-works" {
		t.Errorf("pass-through to 'sh' produced unexpected output: %q, want %q", output, "pass-through-works")
	}
}

// TestPassThroughUnknownBinary verifies that invoking the entrypoint
// with a completely unknown binary name produces an error message
// mentioning the command name, NOT the old "unknown subcommand" error.
func TestPassThroughUnknownBinary(t *testing.T) {
	bin := findEntrypointBinary(t)

	cmd := exec.Command(bin, "nonexistent-binary-xyz-123")
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err == nil {
		t.Fatal("expected error when passing unknown binary, got nil")
	}

	if !strings.Contains(output, "nonexistent-binary-xyz-123") {
		t.Errorf("error output should mention the command name, got:\n%s", output)
	}

	// Must NOT say "unknown subcommand" — the old broken behavior
	if strings.Contains(output, "unknown subcommand: nonexistent") {
		t.Errorf("error output uses old 'unknown subcommand' format, should use new format:\n%s", output)
	}
}

// TestSubcommandStillDispatched verifies that internal subcommands
// are dispatched to their handlers, not treated as pass-through.
func TestSubcommandStillDispatched(t *testing.T) {
	bin := findEntrypointBinary(t)

	// "config" is an internal subcommand. It must NOT be passed through
	// to exec.LookPath — the old error "unknown subcommand or command"
	// would indicate it was incorrectly treated as pass-through.
	cmd := exec.Command(bin, "config")
	out, _ := cmd.CombinedOutput()
	output := string(out)

	if strings.Contains(output, "unknown subcommand or command: config") {
		t.Fatalf("internal subcommand 'config' was incorrectly passed through to exec:\n%s", output)
	}
}

// TestEntrypointNoArgsRunsInit verifies that invoking the entrypoint
// with no arguments runs init (which execs into /bin/sh).
func TestEntrypointNoArgsRunsInit(t *testing.T) {
	bin := findEntrypointBinary(t)

	// No args → init runs, then execs into /bin/sh.
	// We pass "-c echo init-done" via stdin to sh to verify it launched.
	cmd := exec.Command(bin)
	cmd.Stdin = strings.NewReader("echo init-done\nexit\n")
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err != nil {
		t.Logf("entrypoint no-args exited with error (may be expected): %v", err)
	}

	if !strings.Contains(output, "init-done") {
		t.Errorf("entrypoint with no args should exec into sh, expected 'init-done' in output, got:\n%s", output)
	}
}

// findEntrypointBinary locates the pre-built kilo-entrypoint binary.
// It works both on the host and inside the Docker test container.
func findEntrypointBinary(t *testing.T) string {
	t.Helper()

	// Get the directory of this test file
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	testDir := filepath.Dir(filename)

	// From cmd/kilo-entrypoint/ go up two levels to project root/bin/
	relPath := filepath.Join(testDir, "..", "..", "bin", "kilo-entrypoint")
	abs, err := filepath.Abs(relPath)
	if err == nil {
		if _, statErr := filepath.Glob(abs); statErr == nil {
			if _, lookErr := exec.LookPath(abs); lookErr == nil {
				return abs
			}
			// Try executing directly (works even if not in PATH)
			cmd := exec.Command(abs)
			cmd.Stdin = nil
			if runErr := cmd.Run(); runErr == nil {
				return abs
			}
		}
	}

	// Try multiple candidate paths relative to the test file
	candidates := []string{
		relPath,
		"../../bin/kilo-entrypoint",
		"/build/bin/kilo-entrypoint",
	}

	for _, c := range candidates {
		absPath, _ := filepath.Abs(c)
		if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
			return absPath
		}
	}

	// Fallback: try PATH
	if path, err := exec.LookPath("kilo-entrypoint"); err == nil {
		return path
	}

	t.Skip("kilo-entrypoint binary not found; run './scripts/build.sh build-entrypoint' first or skip integration tests")
	return ""
}
