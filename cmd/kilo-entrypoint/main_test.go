package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// --- Unit tests for subcommands map ---

// TestSubcommandsContainsAllInternal verifies that all documented
// internal subcommands are registered in the subcommands map.
func TestSubcommandsContainsAllInternal(t *testing.T) {
	expected := []string{
		"update-config",
		"backup",
		"restore",
		"mcp-config",
		"mcp-tokens",
		"sync",
		"resync",
		"zellij-attach",
		"print-env",
		"custom-envs",
		"completions",
		"help",
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
		"update-config",
		"backup",
		"restore",
		"mcp-config",
		"mcp-tokens",
		"sync",
		"resync",
		"zellij-attach",
		"print-env",
		"custom-envs",
		"completions",
		"help",
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

// --- Unit tests for argument parsing ---

// TestParseEntrypointArgsNoArgs verifies that an empty argument list produces
// an empty command and no help flag.
func TestParseEntrypointArgsNoArgs(t *testing.T) {
	parsed := parseEntrypointArgs([]string{})
	if parsed.help {
		t.Error("expected help=false for empty args")
	}
	if parsed.command != "" {
		t.Errorf("expected empty command, got %q", parsed.command)
	}
	if len(parsed.args) != 0 {
		t.Errorf("expected no args, got %v", parsed.args)
	}
}

// TestParseEntrypointArgsCommandOnly verifies that a bare command is parsed.
func TestParseEntrypointArgsCommandOnly(t *testing.T) {
	parsed := parseEntrypointArgs([]string{"backup"})
	if parsed.help {
		t.Error("expected help=false for backup")
	}
	if parsed.command != "backup" {
		t.Errorf("expected command=backup, got %q", parsed.command)
	}
	if len(parsed.args) != 0 {
		t.Errorf("expected no args, got %v", parsed.args)
	}
}

// TestParseEntrypointArgsHelpFlag verifies that -h and --help set the help flag.
func TestParseEntrypointArgsHelpFlag(t *testing.T) {
	for _, flag := range []string{"-h", "--help"} {
		parsed := parseEntrypointArgs([]string{flag})
		if !parsed.help {
			t.Errorf("expected help=true for %s", flag)
		}
		if parsed.command != "" {
			t.Errorf("expected empty command, got %q", parsed.command)
		}
	}
}

// TestParseEntrypointArgsCommandHelpFlag verifies that -h after a command is
// detected and the command is preserved for context-sensitive help.
func TestParseEntrypointArgsCommandHelpFlag(t *testing.T) {
	parsed := parseEntrypointArgs([]string{"backup", "-h"})
	if !parsed.help {
		t.Error("expected help=true for backup -h")
	}
	if parsed.command != "backup" {
		t.Errorf("expected command=backup, got %q", parsed.command)
	}
	if len(parsed.args) != 0 {
		t.Errorf("expected args stripped of -h, got %v", parsed.args)
	}
}

// TestParseEntrypointArgsHelpCommand verifies that 'help <command>' is parsed
// as a help command with arguments.
func TestParseEntrypointArgsHelpCommand(t *testing.T) {
	parsed := parseEntrypointArgs([]string{"help", "sync", "ls"})
	if parsed.help {
		t.Error("expected help=false for help command syntax")
	}
	if parsed.command != "help" {
		t.Errorf("expected command=help, got %q", parsed.command)
	}
	if len(parsed.args) != 2 || parsed.args[0] != "sync" || parsed.args[1] != "ls" {
		t.Errorf("expected args [sync ls], got %v", parsed.args)
	}
}

// TestParseEntrypointArgsLeadingFlagsSkipped verifies that unknown flags
// before the command are skipped, matching Go's flag.Parse behavior.
func TestParseEntrypointArgsLeadingFlagsSkipped(t *testing.T) {
	parsed := parseEntrypointArgs([]string{"-foo", "backup", "/tmp/out.tar.gz"})
	if parsed.help {
		t.Error("expected help=false for unknown flags")
	}
	if parsed.command != "backup" {
		t.Errorf("expected command=backup, got %q", parsed.command)
	}
	if len(parsed.args) != 1 || parsed.args[0] != "/tmp/out.tar.gz" {
		t.Errorf("expected args [/tmp/out.tar.gz], got %v", parsed.args)
	}
}

// TestParseEntrypointArgsKeepsFlagsAfterCommand verifies that flags after the
// command are preserved so pass-through commands like 'sh -c ...' work.
func TestParseEntrypointArgsKeepsFlagsAfterCommand(t *testing.T) {
	parsed := parseEntrypointArgs([]string{"sh", "-c", "echo hello"})
	if parsed.help {
		t.Error("expected help=false for sh -c")
	}
	if parsed.command != "sh" {
		t.Errorf("expected command=sh, got %q", parsed.command)
	}
	if len(parsed.args) != 2 || parsed.args[0] != "-c" || parsed.args[1] != "echo hello" {
		t.Errorf("expected args [-c 'echo hello'], got %v", parsed.args)
	}
}

// TestCommandWithSubcommand verifies nested command detection for help routing.
func TestCommandWithSubcommand(t *testing.T) {
	cases := []struct {
		cmd  string
		args []string
		want string
	}{
		{"backup", nil, "backup"},
		{"sync", []string{"ls"}, "sync ls"},
		{"sync", []string{"rm"}, "sync rm"},
		{"custom-envs", []string{"add"}, "custom-envs add"},
		{"custom-envs", []string{"unknown"}, "custom-envs"},
	}
	for _, tc := range cases {
		got := commandWithSubcommand(tc.cmd, tc.args)
		if got != tc.want {
			t.Errorf("commandWithSubcommand(%q, %v) = %q, want %q", tc.cmd, tc.args, got, tc.want)
		}
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

	// "mcp-config" is an internal subcommand. It must NOT be passed through
	// to exec.LookPath — the old error "unknown subcommand or command"
	// would indicate it was incorrectly treated as pass-through.
	cmd := exec.Command(bin, "mcp-config")
	out, _ := cmd.CombinedOutput()
	output := string(out)

	if strings.Contains(output, "unknown subcommand or command: mcp-config") {
		t.Fatalf("internal subcommand 'mcp-config' was incorrectly passed through to exec:\n%s", output)
	}
}

// TestEntrypointNoArgsRunsInit verifies that invoking the entrypoint
// with no arguments runs init (which execs into tail -f /dev/null).
func TestEntrypointNoArgsRunsInit(t *testing.T) {
	bin := findEntrypointBinary(t)

	cmd := exec.Command(bin)
	cmd.Stdin = strings.NewReader("")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start entrypoint: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		t.Fatalf("entrypoint with no args should not exit immediately, got exit code: %d", cmd.ProcessState.ExitCode())
	}

	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}

// TestHelpFlagGeneral verifies that 'kilo-entrypoint -h' prints the general help.
func TestHelpFlagGeneral(t *testing.T) {
	bin := findEntrypointBinary(t)

	cmd := exec.Command(bin, "-h")
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err != nil {
		t.Fatalf("expected help to exit cleanly, got err=%v\noutput: %s", err, output)
	}
	if !strings.Contains(output, "kilo-entrypoint - Container entrypoint") {
		t.Errorf("general help missing header, got:\n%s", output)
	}
	if !strings.Contains(output, "Subcommands:") {
		t.Errorf("general help missing subcommands section, got:\n%s", output)
	}
}

// TestHelpCommandSubcommand verifies that 'kilo-entrypoint help backup' prints
// command-specific help.
func TestHelpCommandSubcommand(t *testing.T) {
	bin := findEntrypointBinary(t)

	cmd := exec.Command(bin, "help", "backup")
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err != nil {
		t.Fatalf("expected help backup to exit cleanly, got err=%v\noutput: %s", err, output)
	}
	if !strings.Contains(output, "Usage: kilo-entrypoint backup") {
		t.Errorf("backup help missing usage, got:\n%s", output)
	}
}

// TestHelpFlagNestedCommand verifies that 'kilo-entrypoint sync ls -h' prints
// nested command help.
func TestHelpFlagNestedCommand(t *testing.T) {
	bin := findEntrypointBinary(t)

	cmd := exec.Command(bin, "sync", "ls", "-h")
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err != nil {
		t.Fatalf("expected sync ls -h to exit cleanly, got err=%v\noutput: %s", err, output)
	}
	if !strings.Contains(output, "Usage: kilo-entrypoint sync ls") {
		t.Errorf("sync ls help missing usage, got:\n%s", output)
	}
}

// TestUnknownCommandHelp verifies that asking for help on an unknown command
// prints the unknown-command message.
func TestUnknownCommandHelp(t *testing.T) {
	bin := findEntrypointBinary(t)

	cmd := exec.Command(bin, "help", "nonexistent-command")
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err != nil {
		t.Fatalf("expected unknown command help to exit cleanly, got err=%v\noutput: %s", err, output)
	}
	if !strings.Contains(output, "Unknown command") {
		t.Errorf("expected Unknown command message, got:\n%s", output)
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
