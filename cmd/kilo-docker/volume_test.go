package main

import (
	"strings"
	"testing"
)

func TestDeriveContainerName(t *testing.T) {
	tests := []struct {
		name     string
		pwd      string
		username string
	}{
		{
			name:     "basic workspace and user",
			pwd:      "/home/alice/project",
			username: "alice",
		},
		{
			name:     "different workspace same user",
			pwd:      "/home/alice/other-project",
			username: "alice",
		},
		{
			name:     "same workspace different user",
			pwd:      "/home/alice/project",
			username: "bob",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveContainerName(tt.pwd, tt.username)
			if result == "" {
				t.Error("deriveContainerName returned empty string")
			}
		})
	}
}

func TestDeriveContainerNameHasCorrectPrefix(t *testing.T) {
	result := deriveContainerName("/some/path", "user")
	if !strings.HasPrefix(result, "kilo-") {
		t.Errorf("deriveContainerName() = %q, want prefix 'kilo-'", result)
	}
}

func TestDeriveContainerNameLength(t *testing.T) {
	result := deriveContainerName("/some/path", "user")
	// "kilo-" (5) + 12 hex chars (6 bytes) = 17
	expectedLen := 5 + 12
	if len(result) != expectedLen {
		t.Errorf("deriveContainerName() length = %d, want %d (got %q)", len(result), expectedLen, result)
	}
}

func TestDeriveContainerNameIsDeterministic(t *testing.T) {
	pwd := "/home/alice/project"
	username := "alice"

	first := deriveContainerName(pwd, username)
	second := deriveContainerName(pwd, username)

	if first != second {
		t.Errorf("deriveContainerName() not deterministic: %q != %q", first, second)
	}
}

func TestDeriveContainerNameDiffersForDifferentUsers(t *testing.T) {
	pwd := "/home/alice/project"

	aliceName := deriveContainerName(pwd, "alice")
	bobName := deriveContainerName(pwd, "bob")

	if aliceName == bobName {
		t.Errorf("same container name for different users: %q", aliceName)
	}
}

func TestDeriveContainerNameDiffersForDifferentWorkspaces(t *testing.T) {
	username := "alice"

	name1 := deriveContainerName("/home/alice/project-a", username)
	name2 := deriveContainerName("/home/alice/project-b", username)

	if name1 == name2 {
		t.Errorf("same container name for different workspaces: %q", name1)
	}
}

func TestDeriveContainerNameSeparatorPreventsCollision(t *testing.T) {
	// Without ":" separator, ("abc", "de") and ("abcd", "e") would hash
	// the same input "abcde". With ":" they hash "abc:de" vs "abcd:e".
	name1 := deriveContainerName("abc", "de")
	name2 := deriveContainerName("abcd", "e")

	if name1 == name2 {
		t.Errorf("separator collision: both produced %q (inputs 'abc:de' and 'abcd:e' should differ)", name1)
	}
}

func TestDeriveContainerNameEmptyInputs(t *testing.T) {
	tests := []struct {
		name     string
		pwd      string
		username string
	}{
		{name: "empty pwd", pwd: "", username: "user"},
		{name: "empty username", pwd: "/path", username: ""},
		{name: "both empty", pwd: "", username: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveContainerName(tt.pwd, tt.username)
			if !strings.HasPrefix(result, "kilo-") {
				t.Errorf("deriveContainerName(%q, %q) = %q, want prefix 'kilo-'", tt.pwd, tt.username, result)
			}
			if len(result) != 17 {
				t.Errorf("deriveContainerName(%q, %q) length = %d, want 17", tt.pwd, tt.username, len(result))
			}
		})
	}
}
