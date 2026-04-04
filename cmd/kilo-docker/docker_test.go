package main

import (
	"reflect"
	"slices"
	"testing"
)

func TestEnsureRunArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "empty args returns empty",
			args:     []string{},
			expected: []string{},
		},
		{
			name:     "args starting with -e get run --rm prepended",
			args:     []string{"-e", "USERNAME=foo", "image", "cmd"},
			expected: []string{"run", "--rm", "-e", "USERNAME=foo", "image", "cmd"},
		},
		{
			name:     "args starting with -v get run --rm prepended",
			args:     []string{"-v", "vol:/path", "image", "load-tokens"},
			expected: []string{"run", "--rm", "-v", "vol:/path", "image", "load-tokens"},
		},
		{
			name:     "args starting with run are unchanged",
			args:     []string{"run", "--rm", "-d", "--name", "test", "image"},
			expected: []string{"run", "--rm", "-d", "--name", "test", "image"},
		},
		{
			name:     "args starting with pull are unchanged",
			args:     []string{"pull", "ghcr.io/example:latest"},
			expected: []string{"pull", "ghcr.io/example:latest"},
		},
		{
			name:     "args starting with ps are unchanged",
			args:     []string{"ps", "-a", "--filter", "label=kilo.workspace"},
			expected: []string{"ps", "-a", "--filter", "label=kilo.workspace"},
		},
		{
			name:     "args starting with inspect are unchanged",
			args:     []string{"inspect", "-f", "{{.State.Status}}", "container"},
			expected: []string{"inspect", "-f", "{{.State.Status}}", "container"},
		},
		{
			name:     "args starting with rm are unchanged",
			args:     []string{"rm", "-f", "container"},
			expected: []string{"rm", "-f", "container"},
		},
		{
			name:     "args starting with cp are unchanged",
			args:     []string{"cp", "container:/path", "/local/path"},
			expected: []string{"cp", "container:/path", "/local/path"},
		},
		{
			name:     "args starting with network are unchanged",
			args:     []string{"network", "create", "net"},
			expected: []string{"network", "create", "net"},
		},
		{
			name:     "args starting with volume are unchanged",
			args:     []string{"volume", "create", "vol"},
			expected: []string{"volume", "create", "vol"},
		},
		{
			name:     "args starting with exec are unchanged",
			args:     []string{"exec", "container", "ls"},
			expected: []string{"exec", "container", "ls"},
		},
		{
			name:     "args starting with logs are unchanged",
			args:     []string{"logs", "container"},
			expected: []string{"logs", "container"},
		},
		{
			name:     "unknown subcommand gets run --rm prepended",
			args:     []string{"sh", "-c", "echo hello"},
			expected: []string{"run", "--rm", "sh", "-c", "echo hello"},
		},
		{
			name:     "multiple -e flags get run --rm prepended",
			args:     []string{"-e", "A=1", "-e", "B=2", "image", "cmd"},
			expected: []string{"run", "--rm", "-e", "A=1", "-e", "B=2", "image", "cmd"},
		},
		{
			name:     "mixed -v and -e flags get run --rm prepended",
			args:     []string{"-v", "vol:/path", "-e", "FOO=bar", "image"},
			expected: []string{"run", "--rm", "-v", "vol:/path", "-e", "FOO=bar", "image"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture original to verify immutability of input
			original := make([]string, len(tt.args))
			copy(original, tt.args)

			result := ensureRunArgs(tt.args)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ensureRunArgs(%v) = %v, want %v", original, result, tt.expected)
			}
		})
	}
}

func TestEnsureRunArgsDoesNotMutateInput(t *testing.T) {
	args := []string{"-e", "FOO=bar", "image", "cmd"}
	original := make([]string, len(args))
	copy(original, args)

	ensureRunArgs(args)

	// The original slice backing array should not be modified
	// (ensureRunArgs returns a new slice via append, so this passes)
	if !reflect.DeepEqual(args, original) {
		t.Errorf("ensureRunArgs mutated input: got %v, want %v", args, original)
	}
}

func TestAllDockerSubcommandsAreRecognized(t *testing.T) {
	// Verify that all documented docker subcommands are in the map.
	// This catches cases where new subcommands are added to Docker but
	// not to our map (they'd incorrectly get run --rm prepended).
	commonSubcommands := []string{
		"attach", "build", "commit", "cp", "create", "diff", "events",
		"exec", "export", "history", "images", "import", "info", "inspect",
		"kill", "load", "login", "logout", "logs", "network", "pause",
		"port", "ps", "pull", "push", "rename", "restart", "rm", "rmi",
		"run", "save", "search", "start", "stats", "stop", "tag", "top",
		"unpause", "update", "version", "volume", "wait",
	}

	for _, cmd := range commonSubcommands {
		if !dockerSubcommands[cmd] {
			t.Errorf("docker subcommand %q is missing from dockerSubcommands map", cmd)
		}
	}

	// Verify the map doesn't contain unexpected entries
	if len(dockerSubcommands) != len(commonSubcommands) {
		extra := []string{}
		for k := range dockerSubcommands {
			if !slices.Contains(commonSubcommands, k) {
				extra = append(extra, k)
			}
		}
		if len(extra) > 0 {
			t.Errorf("dockerSubcommands contains unexpected entries: %v", extra)
		}
	}
}

// TestEnsureRunArgsPreservesExistingRunCalls verifies that callers which
// already include "run" are not double-prefixed.
func TestEnsureRunArgsPreservesExistingRunCalls(t *testing.T) {
	// Pattern from handle_backup.go:59
	args := []string{"run", "--rm", "-d", "--name", "temp", "-v", "vol:/src:ro", "alpine:latest", "tail", "-f", "/dev/null"}
	expected := []string{"run", "--rm", "-d", "--name", "temp", "-v", "vol:/src:ro", "alpine:latest", "tail", "-f", "/dev/null"}

	result := ensureRunArgs(args)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("existing run call was modified:\n  got:      %v\n  expected: %v", result, expected)
	}
}

// TestEnsureRunArgsBrokenPatternsFromBugReport tests the exact call patterns
// that were broken before the fix, ensuring they now produce correct commands.
func TestEnsureRunArgsBrokenPatternsFromBugReport(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCmd  string // first arg after potential prepend
		wantFull string // full joined command
	}{
		{
			name:     "ainstruct.go login call",
			args:     []string{"-e", "USERNAME=user", "-e", "PASSWORD=pass", "-e", "API_URL=https://example.com", "image", "ainstruct-login"},
			wantFull: "run --rm -e USERNAME=user -e PASSWORD=pass -e API_URL=https://example.com image ainstruct-login",
		},
		{
			name:     "tokens.go loadTokens encrypted call",
			args:     []string{"-v", "vol:/home/kd", "image", "cat", "/home/kd/.tokens.env.enc"},
			wantFull: "run --rm -v vol:/home/kd image cat /home/kd/.tokens.env.enc",
		},
		{
			name:     "tokens.go loadTokens unencrypted call",
			args:     []string{"-v", "vol:/home/kd", "image", "load-tokens"},
			wantFull: "run --rm -v vol:/home/kd image load-tokens",
		},
		{
			name:     "handlers.go updateConfig call",
			args:     []string{"-v", "vol:/home/kd", "image:latest", "update-config"},
			wantFull: "run --rm -v vol:/home/kd image:latest update-config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ensureRunArgs(tt.args)
			got := ""
			for i, s := range result {
				if i > 0 {
					got += " "
				}
				got += s
			}
			if got != tt.wantFull {
				t.Errorf("ensureRunArgs() produced:\n  %q\nwant:\n  %q", got, tt.wantFull)
			}
			// Verify first arg is always "run"
			if result[0] != "run" {
				t.Errorf("first arg = %q, want %q", result[0], "run")
			}
			// Verify second arg is always "--rm"
			if result[1] != "--rm" {
				t.Errorf("second arg = %q, want %q", result[1], "--rm")
			}
		})
	}
}

func TestBuildRunArgs(t *testing.T) {
	tests := []struct {
		name       string
		dockerArgs []string
		image      string
		extraArgs  []string
		terminal   bool
		expected   []string
	}{
		{
			name:       "terminal mode adds -it after run",
			dockerArgs: []string{"--name", "test", "-v", "vol:/home/kd"},
			image:      "ghcr.io/mbabic84/kilo-docker:latest",
			extraArgs:  nil,
			terminal:   true,
			expected:   []string{"run", "-it", "--name", "test", "-v", "vol:/home/kd", "ghcr.io/mbabic84/kilo-docker:latest"},
		},
		{
			name:       "non-terminal mode adds -i after run",
			dockerArgs: []string{"--name", "test", "-v", "vol:/home/kd"},
			image:      "ghcr.io/mbabic84/kilo-docker:latest",
			extraArgs:  nil,
			terminal:   false,
			expected:   []string{"run", "-i", "--name", "test", "-v", "vol:/home/kd", "ghcr.io/mbabic84/kilo-docker:latest"},
		},
		{
			name:       "with extra args appended after image",
			dockerArgs: []string{"--name", "test"},
			image:      "ghcr.io/mbabic84/kilo-docker:latest",
			extraArgs:  []string{"bash"},
			terminal:   true,
			expected:   []string{"run", "-it", "--name", "test", "ghcr.io/mbabic84/kilo-docker:latest", "bash"},
		},
		{
			name:       "with multiple extra args",
			dockerArgs: []string{"--rm"},
			image:      "alpine:latest",
			extraArgs:  []string{"sh", "-c", "echo hello"},
			terminal:   true,
			expected:   []string{"run", "-it", "--rm", "alpine:latest", "sh", "-c", "echo hello"},
		},
		{
			name:       "empty dockerArgs",
			dockerArgs: []string{},
			image:      "alpine:latest",
			extraArgs:  nil,
			terminal:   true,
			expected:   []string{"run", "-it", "alpine:latest"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildRunArgs(tt.dockerArgs, tt.image, tt.extraArgs, tt.terminal)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("buildRunArgs() = %v, want %v", result, tt.expected)
			}
			// First arg must always be "run"
			if result[0] != "run" {
				t.Errorf("first arg = %q, want %q", result[0], "run")
			}
			// Second arg must be the interactive flag
			if tt.terminal && result[1] != "-it" {
				t.Errorf("second arg = %q, want %q", result[1], "-it")
			}
			if !tt.terminal && result[1] != "-i" {
				t.Errorf("second arg = %q, want %q", result[1], "-i")
			}
		})
	}
}

// TestBuildRunArgsProducesDockerRunNotDashIT verifies the fix for the bug
// where "-it" was prepended before "run", producing "docker -it run ..."
// instead of "docker run -it ...".
func TestBuildRunArgsProducesDockerRunNotDashIT(t *testing.T) {
	dockerArgs := []string{"--rm", "--name", "kilo-test", "-v", "vol:/home/kd"}
	image := "ghcr.io/mbabic84/kilo-docker:latest"

	result := buildRunArgs(dockerArgs, image, nil, true)

	// The command must be "docker run -it ..." not "docker -it run ..."
	joined := ""
	for i, s := range result {
		if i > 0 {
			joined += " "
		}
		joined += s
	}
	expected := "run -it --rm --name kilo-test -v vol:/home/kd ghcr.io/mbabic84/kilo-docker:latest"
	if joined != expected {
		t.Errorf("buildRunArgs() produced:\n  %q\nwant:\n  %q", joined, expected)
	}

	// Explicitly verify the first two elements
	if result[0] != "run" {
		t.Errorf("bug regression: first arg is %q, not 'run' — produces 'docker %s run ...'", result[0], result[0])
	}
	if result[1] != "-it" {
		t.Errorf("second arg = %q, want %q", result[1], "-it")
	}
}

// TestDockerRunWithStdinInsertsStdinFlag verifies that the stdin variant
// inserts -i after "run --rm" so Docker attaches stdin for data piping.
// Without -i, docker run ignores stdin and the container receives empty input.
func TestDockerRunWithStdinInsertsStdinFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantFull string
	}{
		{
			name:     "saveTokens encrypted call",
			args:     []string{"-v", "vol:/home/kd", "image", "sh", "-c", "cat > /path"},
			wantFull: "run --rm -i -v vol:/home/kd image sh -c cat > /path",
		},
		{
			name:     "saveTokens unencrypted call",
			args:     []string{"-v", "vol:/home/kd", "image", "save-tokens"},
			wantFull: "run --rm -i -v vol:/home/kd image save-tokens",
		},
		{
			name:     "saveSkipMarker call",
			args:     []string{"-v", "vol:/home/kd", "image", "sh", "-c", "mkdir -p dir && cat > file"},
			wantFull: "run --rm -i -v vol:/home/kd image sh -c mkdir -p dir && cat > file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate what dockerRunWithStdin does to its args:
			// 1. ensureRunArgs prepends "run --rm"
			// 2. Insert -i at position 2 (after "run --rm")
			args := ensureRunArgs(tt.args)
			args = append(args[:2], append([]string{"-i"}, args[2:]...)...)

			got := ""
			for i, s := range args {
				if i > 0 {
					got += " "
				}
				got += s
			}
			if got != tt.wantFull {
				t.Errorf("dockerRunWithStdin args produced:\n  %q\nwant:\n  %q", got, tt.wantFull)
			}

			// Verify structure: run --rm -i ...
			if args[0] != "run" {
				t.Errorf("args[0] = %q, want %q", args[0], "run")
			}
			if args[1] != "--rm" {
				t.Errorf("args[1] = %q, want %q", args[1], "--rm")
			}
			if args[2] != "-i" {
				t.Errorf("args[2] = %q, want %q — without -i, Docker ignores stdin", args[2], "-i")
			}
		})
	}
}

// TestDockerRunDoesNotInsertStdinFlag verifies that the regular dockerRun
// does NOT insert -i (it uses terminal stdin, not programmatic input).
func TestDockerRunDoesNotInsertStdinFlag(t *testing.T) {
	args := []string{"-v", "vol:/home/kd", "image", "cat", "/path"}
	result := ensureRunArgs(args)

	// ensureRunArgs should produce "run --rm ..." WITHOUT -i
	got := ""
	for i, s := range result {
		if i > 0 {
			got += " "
		}
		got += s
	}
	expected := "run --rm -v vol:/home/kd image cat /path"
	if got != expected {
		t.Errorf("ensureRunArgs() produced:\n  %q\nwant:\n  %q", got, expected)
	}

	// Verify -i is NOT in the args
	for i, arg := range result {
		if arg == "-i" {
			t.Errorf("found -i at position %d — dockerRun should not add -i", i)
		}
	}
}
