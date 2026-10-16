package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// withTempMarkers redirects the init markers to a temp directory and restores
// them (plus the claim flag) after the test.
func withTempMarkers(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	oldInit, oldInProgress, oldLock := initMarker, userInitInProgressMarker, userInitLockFile
	oldClaimed := userInitClaimed.Load()
	initMarker = filepath.Join(dir, "initialized")
	userInitInProgressMarker = filepath.Join(dir, "in-progress")
	userInitLockFile = filepath.Join(dir, "init.lock")
	t.Cleanup(func() {
		initMarker = oldInit
		userInitInProgressMarker = oldInProgress
		userInitLockFile = oldLock
		userInitClaimed.Store(oldClaimed)
	})
}

// absentPID is a PID above the usual pid_max range, so /proc/<pid> does not
// exist and the process is provably gone.
const absentPID = 1 << 30

func TestInitProcAlive(t *testing.T) {
	start, err := readProcStartTime(os.Getpid())
	if err != nil {
		t.Fatalf("readProcStartTime(self): %v", err)
	}
	if !initProcAlive(os.Getpid(), start) {
		t.Error("expected own process to be reported alive")
	}
	if initProcAlive(os.Getpid(), start+1) {
		t.Error("expected start-time mismatch to be reported not alive")
	}
	if initProcAlive(absentPID, 1) {
		t.Error("expected absent PID to be reported not alive")
	}
}

func TestParseInitMarkerRoundTrip(t *testing.T) {
	withTempMarkers(t)
	f, err := os.OpenFile(userInitInProgressMarker, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeInitMarker(f); err != nil {
		t.Fatalf("writeInitMarker: %v", err)
	}
	_ = f.Close()

	data, err := os.ReadFile(userInitInProgressMarker)
	if err != nil {
		t.Fatal(err)
	}
	info, err := parseInitMarker(string(data))
	if err != nil {
		t.Fatalf("parseInitMarker(%q): %v", string(data), err)
	}
	if info.pid != os.Getpid() {
		t.Errorf("pid = %d, want %d", info.pid, os.Getpid())
	}
}

func TestStaleInitMarker(t *testing.T) {
	self, err := readProcStartTime(os.Getpid())
	if err != nil {
		t.Fatalf("readProcStartTime(self): %v", err)
	}

	cases := []struct {
		name string
		data string
		want bool
	}{
		{"live owner", fmt.Sprintf("%d %d\n", os.Getpid(), self), false},
		{"dead owner", fmt.Sprintf("%d 1\n", absentPID), true},
		{"pid reuse", fmt.Sprintf("%d %d\n", os.Getpid(), self+1), true},
		{"garbage", "not-a-marker", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		if got := staleInitMarker(tc.data); got != tc.want {
			t.Errorf("staleInitMarker(%s) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestClaimUserInitWritesParseableMarkerAtomically(t *testing.T) {
	withTempMarkers(t)

	claimed, err := tryClaimUserInit()
	if err != nil {
		t.Fatalf("tryClaimUserInit: %v", err)
	}
	if !claimed {
		t.Fatal("expected to claim an absent marker")
	}

	data, err := os.ReadFile(userInitInProgressMarker)
	if err != nil {
		t.Fatal(err)
	}
	info, err := parseInitMarker(string(data))
	if err != nil {
		t.Fatalf("marker must be parseable immediately after claim, got %q: %v", string(data), err)
	}
	if info.pid != os.Getpid() {
		t.Errorf("marker pid = %d, want %d", info.pid, os.Getpid())
	}
}

func TestClaimUserInitReclaimsStaleMarker(t *testing.T) {
	withTempMarkers(t)
	if err := os.WriteFile(userInitInProgressMarker, []byte(fmt.Sprintf("%d 1\n", absentPID)), 0o600); err != nil {
		t.Fatal(err)
	}

	claimed, err := claimUserInit()
	if err != nil {
		t.Fatalf("claimUserInit: %v", err)
	}
	if !claimed {
		t.Fatal("expected claim after reclaiming stale marker")
	}

	data, err := os.ReadFile(userInitInProgressMarker)
	if err != nil {
		t.Fatal(err)
	}
	info, err := parseInitMarker(string(data))
	if err != nil {
		t.Fatalf("reclaimed marker unparseable: %v", err)
	}
	if info.pid != os.Getpid() {
		t.Errorf("reclaimed marker pid = %d, want %d", info.pid, os.Getpid())
	}
}

func TestClaimUserInitKeepsLiveMarker(t *testing.T) {
	withTempMarkers(t)
	self, err := readProcStartTime(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userInitInProgressMarker, []byte(fmt.Sprintf("%d %d\n", os.Getpid(), self)), 0o600); err != nil {
		t.Fatal(err)
	}

	claimed, err := claimUserInit()
	if err != nil {
		t.Fatalf("claimUserInit: %v", err)
	}
	if claimed {
		t.Error("must not claim a marker owned by a live process")
	}
}

func TestClaimUserInitKeepsUnparseableMarker(t *testing.T) {
	withTempMarkers(t)
	if err := os.WriteFile(userInitInProgressMarker, []byte("legacy-format\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	claimed, err := claimUserInit()
	if err != nil {
		t.Fatalf("claimUserInit: %v", err)
	}
	if claimed {
		t.Error("must not reclaim a marker whose owner cannot be determined")
	}
}

func TestWaitForCompletedUserInitStaleMarker(t *testing.T) {
	withTempMarkers(t)
	if err := os.WriteFile(userInitInProgressMarker, []byte(fmt.Sprintf("%d 1\n", absentPID)), 0o600); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	err := waitForCompletedUserInit(30 * time.Second)
	if !errors.Is(err, errStaleUserInitMarker) {
		t.Fatalf("expected stale marker error, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("stale detection took %s; must not block for the full timeout", elapsed)
	}
}

func TestAcquireUserInitReclaimsStaleMarker(t *testing.T) {
	withTempMarkers(t)
	if err := os.WriteFile(userInitInProgressMarker, []byte(fmt.Sprintf("%d 1\n", absentPID)), 0o600); err != nil {
		t.Fatal(err)
	}

	claimed, err := acquireUserInit(5 * time.Second)
	if err != nil {
		t.Fatalf("acquireUserInit: %v", err)
	}
	if !claimed {
		t.Fatal("expected acquireUserInit to claim after reclaiming the stale marker")
	}
}

func TestAcquireUserInitWaitsForLiveOwner(t *testing.T) {
	withTempMarkers(t)
	self, err := readProcStartTime(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userInitInProgressMarker, []byte(fmt.Sprintf("%d %d\n", os.Getpid(), self)), 0o600); err != nil {
		t.Fatal(err)
	}

	claimed, err := acquireUserInit(150 * time.Millisecond)
	if err == nil {
		t.Fatal("expected a timeout waiting for a live owner")
	}
	if errors.Is(err, errStaleUserInitMarker) {
		t.Errorf("live owner must not be reported as a stale marker: %v", err)
	}
	if claimed {
		t.Error("must not claim while a live owner holds the marker")
	}
}
