package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInitReadyTimeoutDefault(t *testing.T) {
	_ = os.Unsetenv(initReadyTimeoutEnvVar)

	if got := initReadyTimeout(); got != defaultInitReadyTimeout {
		t.Fatalf("initReadyTimeout() = %s, want %s", got, defaultInitReadyTimeout)
	}
}

func TestInitReadyTimeoutOverride(t *testing.T) {
	t.Setenv(initReadyTimeoutEnvVar, "90s")

	if got := initReadyTimeout(); got != 90*time.Second {
		t.Fatalf("initReadyTimeout() = %s, want %s", got, 90*time.Second)
	}
}

func TestInitReadyTimeoutInvalidFallsBack(t *testing.T) {
	t.Setenv(initReadyTimeoutEnvVar, "not-a-duration")

	if got := initReadyTimeout(); got != defaultInitReadyTimeout {
		t.Fatalf("initReadyTimeout() = %s, want fallback %s", got, defaultInitReadyTimeout)
	}
}

func TestInitReadyTimeoutNonPositiveFallsBack(t *testing.T) {
	t.Setenv(initReadyTimeoutEnvVar, "0s")

	if got := initReadyTimeout(); got != defaultInitReadyTimeout {
		t.Fatalf("initReadyTimeout() = %s, want fallback %s", got, defaultInitReadyTimeout)
	}
}

func TestWaitForMarkerReturnsWhenMarkerAppears(t *testing.T) {
	marker := filepath.Join(t.TempDir(), ".marker")

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.WriteFile(marker, []byte("1\n"), 0o600)
	}()

	if err := waitForMarker(marker, 500*time.Millisecond, "waiting\n"); err != nil {
		t.Fatalf("waitForMarker() error = %v", err)
	}
}

func TestWaitForMarkerTimesOut(t *testing.T) {
	marker := filepath.Join(t.TempDir(), ".missing-marker")

	err := waitForMarker(marker, 100*time.Millisecond, "waiting\n")
	if err == nil {
		t.Fatal("waitForMarker() error = nil, want timeout")
	}

	if got := err.Error(); got == "" {
		t.Fatal("waitForMarker() returned empty error")
	}
}
