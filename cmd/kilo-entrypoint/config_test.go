package main

import (
	"encoding/json"
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
