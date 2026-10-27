package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStripJSONCComments(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "line comment does not swallow following lines",
			in:   "{\n  \"a\": 1,\n  // comment\n  \"b\": 2\n}",
			want: "{\n  \"a\": 1,\n  \n  \"b\": 2\n}",
		},
		{
			name: "multiple line comments",
			in:   "{\n  // one\n  // two\n  \"a\": 1\n}",
			want: "{\n  \n  \n  \"a\": 1\n}",
		},
		{
			name: "block comment across lines",
			in:   "{\n  /* a\n  b */\n  \"a\": 1\n}",
			want: "{\n  \n  \"a\": 1\n}",
		},
		{
			name: "comment markers inside strings are preserved",
			in:   `{"url": "http://example.com/mcp", "pat": "gh api */repos/*/contents/*"}`,
			want: `{"url": "http://example.com/mcp", "pat": "gh api */repos/*/contents/*"}`,
		},
		{
			name: "escaped quotes inside strings are preserved",
			in:   `{"a": "he said \"// hi\"", "b": 1}`,
			want: `{"a": "he said \"// hi\"", "b": 1}`,
		},
		{
			name: "trailing line comment without newline",
			in:   "{\"a\": 1}\n// trailing",
			want: "{\"a\": 1}\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(stripJSONCComments([]byte(tc.in)))
			if got != tc.want {
				t.Fatalf("stripJSONCComments() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestStripJSONCCommentsParsesRealisticConfig guards the regression that
// produced "unexpected end of JSON input": a `//` comment inside the
// permission block truncated everything after it, so the remaining text
// was not valid JSON.
func TestStripJSONCCommentsParsesRealisticConfig(t *testing.T) {
	in := `{
  "$schema": "https://app.kilo.ai/config.json",
  "mcp": {
    "ainstruct": {
      "type": "remote",
      "url": "{env:KD_AINSTRUCT_BASE_URL}/mcp",
      "enabled": false
    },
    "github": {
      "type": "local",
      "command": ["gh", "mcp"],
      "enabled": false
    }
  },
  "permission": {
    "bash": {
      // Rules are evaluated in order; the last matching pattern wins.
      // Keep broad rules first and narrow exceptions after them.
      "*": "ask",
      "./bin/kilo-entrypoint *": "allow",
      "gh api */repos/*/contents/*": "allow",
      "curl *--output /dev/null*": "allow"
    }
  }
}`

	var cfg map[string]any
	if err := json.Unmarshal(stripJSONCComments([]byte(in)), &cfg); err != nil {
		t.Fatalf("stripJSONCComments output is not valid JSON: %v", err)
	}

	perm, ok := cfg["permission"].(map[string]any)
	if !ok {
		t.Fatal("permission section missing")
	}
	bash, ok := perm["bash"].(map[string]any)
	if !ok {
		t.Fatal("permission.bash section missing")
	}
	if v, ok := bash["curl *--output /dev/null*"]; !ok || v != "allow" {
		t.Fatalf("entry after line comment was lost: %v", bash["curl *--output /dev/null*"])
	}
}

func TestMCPEnabledStatesGiteaMatrix(t *testing.T) {
	t.Setenv("PLAYWRIGHT_ENABLED", "")
	t.Setenv("GITNEXUS_ENABLED", "")

	tests := []struct {
		name     string
		services string
		token    string
		want     bool
	}{
		{"service and token", "gitea", "tok", true},
		{"service with other services and token", "go,gitea", "tok", true},
		{"service without token", "gitea", "", false},
		{"token without service", "go", "tok", false},
		{"similar service name does not match", "gitea-mcp", "tok", false},
		{"empty", "", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := TokenStore{}
			if tc.token != "" {
				store.Set(tokenKeyGiteaToken, tc.token)
			}
			enabled := mcpEnabledStates(tc.services, store)
			if got := enabled["gitea"]; got != tc.want {
				t.Errorf("gitea enabled = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMCPEnabledStatesStillUsesStoredMCPTokens(t *testing.T) {
	t.Setenv("PLAYWRIGHT_ENABLED", "1")
	t.Setenv("GITNEXUS_ENABLED", "")

	store := TokenStore{}
	store.Set(tokenKeyContext7, "ctx7")
	store.Set(tokenKeyAinstruct, "ainstruct")

	enabled := mcpEnabledStates("gh", store)

	if !enabled["context7"] {
		t.Error("context7 should be enabled when a token is stored")
	}
	if !enabled["ainstruct"] {
		t.Error("ainstruct should be enabled when a token is stored")
	}
	if !enabled["github"] {
		t.Error("github should be enabled when gh service is active")
	}
	if !enabled["playwright"] {
		t.Error("playwright should follow PLAYWRIGHT_ENABLED")
	}
}

func TestEnsureMCPEntryInsertsGiteaDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "kilo.jsonc")
	if err := os.WriteFile(configPath, []byte(`{"mcp": {}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		if err := ensureMCPEntry(configPath, "gitea", mcpServerDefaults["gitea"]); err != nil {
			t.Fatalf("ensureMCPEntry() call %d error = %v", i+1, err)
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("config is not valid JSON: %v", err)
	}
	mcp, ok := cfg["mcp"].(map[string]any)
	if !ok {
		t.Fatal("mcp section missing")
	}
	entry, ok := mcp["gitea"].(map[string]any)
	if !ok {
		t.Fatal("gitea entry missing")
	}
	if entry["type"] != "local" {
		t.Errorf("type = %v, want local", entry["type"])
	}
	command, ok := entry["command"].([]any)
	if !ok || len(command) != 3 || command[0] != "gitea-mcp" || command[1] != "-t" || command[2] != "stdio" {
		t.Errorf("command = %v, want [gitea-mcp -t stdio]", entry["command"])
	}
	if entry["enabled"] != false {
		t.Errorf("enabled = %v, want false", entry["enabled"])
	}
	environment, ok := entry["environment"].(map[string]any)
	if !ok {
		t.Fatalf("environment = %v, want object", entry["environment"])
	}
	wantEnv := map[string]string{
		"GITEA_HOST":         "{env:KD_GITEA_HOST}",
		"GITEA_ACCESS_TOKEN": "{env:KD_MCP_GITEA_TOKEN}",
		"GITEA_INSECURE":     "{env:KD_GITEA_INSECURE}",
	}
	for key, want := range wantEnv {
		if environment[key] != want {
			t.Errorf("environment[%s] = %v, want %q", key, environment[key], want)
		}
	}
}
