package main

import (
	"os"
	"testing"
)

func TestSetContainerIdentityEnv_UsesHostKDWorkspace(t *testing.T) {
	t.Setenv("KD_WORKSPACE", "/host/workspace")
	// Simulate that HOME was already chdir'd; ensure helper does not
	// overwrite KD_WORKSPACE with os.Getwd().
	t.Chdir("/")

	setContainerIdentityEnv("kd-abc123", "/home/kd-abc123", "[test]")

	if got := os.Getenv("KD_CONTAINER_USER"); got != "kd-abc123" {
		t.Errorf("KD_CONTAINER_USER = %q, want %q", got, "kd-abc123")
	}
	if got := os.Getenv("KD_CONTAINER_HOME"); got != "/home/kd-abc123" {
		t.Errorf("KD_CONTAINER_HOME = %q, want %q", got, "/home/kd-abc123")
	}
	if got := os.Getenv("KD_WORKSPACE"); got != "/host/workspace" {
		t.Errorf("KD_WORKSPACE = %q, want %q (host value should win over Getwd)", got, "/host/workspace")
	}
}

func TestSetContainerIdentityEnv_FallsBackToGetwd(t *testing.T) {
	// Unset so the fallback path fires.
	if err := os.Unsetenv("KD_WORKSPACE"); err != nil {
		t.Fatal(err)
	}
	// t.Chdir requires the target to exist.
	fallbackDir := t.TempDir()
	t.Chdir(fallbackDir)

	setContainerIdentityEnv("kd-xyz789", "/home/kd-xyz789", "[test]")

	if got := os.Getenv("KD_CONTAINER_USER"); got != "kd-xyz789" {
		t.Errorf("KD_CONTAINER_USER = %q, want %q", got, "kd-xyz789")
	}
	if got := os.Getenv("KD_CONTAINER_HOME"); got != "/home/kd-xyz789" {
		t.Errorf("KD_CONTAINER_HOME = %q, want %q", got, "/home/kd-xyz789")
	}
	if got := os.Getenv("KD_WORKSPACE"); got != fallbackDir {
		t.Errorf("KD_WORKSPACE = %q, want %q (fallback to Getwd)", got, fallbackDir)
	}
}

func TestSetContainerIdentityEnv_ReattachMirrorsUserInit(t *testing.T) {
	// Regression guard: re-attach path must set the same vars as the
	// first-time init path. Both callers (runUserInit, ensureSyncForCurrentUser)
	// funnel through setContainerIdentityEnv, so this test exercises the
	// shared contract.
	t.Setenv("KD_WORKSPACE", "/host/project")

	setContainerIdentityEnv("kd-shared", "/home/kd-shared", "[zellijattach]")

	for _, v := range []struct{ name, want string }{
		{"KD_CONTAINER_USER", "kd-shared"},
		{"KD_CONTAINER_HOME", "/home/kd-shared"},
		{"KD_WORKSPACE", "/host/project"},
	} {
		if got := os.Getenv(v.name); got != v.want {
			t.Errorf("%s = %q, want %q", v.name, got, v.want)
		}
	}
}
