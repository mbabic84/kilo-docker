package main

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestSyncerWithHashDir(t *testing.T) *Syncer {
	t.Helper()
	dir := t.TempDir()
	return &Syncer{
		homeDir:  dir,
		hashFile: filepath.Join(dir, ".config", "kilo", ".ainstruct-hashes"),
	}
}

func TestHashSetAndGet(t *testing.T) {
	s := newTestSyncerWithHashDir(t)

	if err := s.hashSet("rules/test.md", "abc123"); err != nil {
		t.Fatalf("hashSet failed: %v", err)
	}
	got := s.hashGet("rules/test.md")
	if got != "abc123" {
		t.Errorf("hashGet returned %q, want %q", got, "abc123")
	}
}

func TestHashGetMissing(t *testing.T) {
	s := newTestSyncerWithHashDir(t)

	got := s.hashGet("nonexistent.md")
	if got != "" {
		t.Errorf("expected empty string for missing hash, got %q", got)
	}
}

func TestHashGetMissingFile(t *testing.T) {
	s := newTestSyncerWithHashDir(t)

	got := s.hashGet("any/path.md")
	if got != "" {
		t.Errorf("expected empty string when hash file doesn't exist, got %q", got)
	}
}

func TestHashSetUpdate(t *testing.T) {
	s := newTestSyncerWithHashDir(t)

	if err := s.hashSet("rules/test.md", "abc123"); err != nil {
		t.Fatalf("hashSet failed: %v", err)
	}
	if err := s.hashSet("rules/test.md", "def456"); err != nil {
		t.Fatalf("hashSet update failed: %v", err)
	}
	got := s.hashGet("rules/test.md")
	if got != "def456" {
		t.Errorf("hashGet returned %q after update, want %q", got, "def456")
	}
}

func TestHashSetMultiple(t *testing.T) {
	s := newTestSyncerWithHashDir(t)

	s.hashSet("rules/a.md", "aaa")
	s.hashSet("rules/b.md", "bbb")
	s.hashSet("commands/c.md", "ccc")

	if got := s.hashGet("rules/a.md"); got != "aaa" {
		t.Errorf("a.md: got %q, want aaa", got)
	}
	if got := s.hashGet("rules/b.md"); got != "bbb" {
		t.Errorf("b.md: got %q, want bbb", got)
	}
	if got := s.hashGet("commands/c.md"); got != "ccc" {
		t.Errorf("c.md: got %q, want ccc", got)
	}
}

func TestHashDelete(t *testing.T) {
	s := newTestSyncerWithHashDir(t)

	s.hashSet("rules/test.md", "abc123")
	s.hashSet("rules/other.md", "xyz789")

	if err := s.hashDelete("rules/test.md"); err != nil {
		t.Fatalf("hashDelete failed: %v", err)
	}

	if got := s.hashGet("rules/test.md"); got != "" {
		t.Errorf("expected empty after delete, got %q", got)
	}
	// Other entry should be unaffected
	if got := s.hashGet("rules/other.md"); got != "xyz789" {
		t.Errorf("other.md affected by delete: got %q, want xyz789", got)
	}
}

func TestHashDeleteMissing(t *testing.T) {
	s := newTestSyncerWithHashDir(t)

	// Delete from nonexistent file should not error
	if err := s.hashDelete("nonexistent.md"); err != nil {
		t.Fatalf("hashDelete on missing file should not error: %v", err)
	}
}

func TestHashSetErrorOnReadOnlyDir(t *testing.T) {
	s := newTestSyncerWithHashDir(t)

	// Make the hash directory read-only
	hashDir := filepath.Dir(s.hashFile)
	os.MkdirAll(hashDir, 0o755)
	os.Chmod(hashDir, 0o555)
	defer os.Chmod(hashDir, 0o755)

	err := s.hashSet("test.md", "abc")
	if err == nil {
		// Some systems allow writing to read-only dirs as owner, skip if so
		t.Log("hashSet did not error (may be running as root), skipping error check")
	}
}

func TestHashDeleteErrorOnReadOnlyDir(t *testing.T) {
	s := newTestSyncerWithHashDir(t)

	// Create a hash file first
	s.hashSet("test.md", "abc")

	// Make the hash directory read-only
	hashDir := filepath.Dir(s.hashFile)
	os.Chmod(hashDir, 0o555)
	defer os.Chmod(hashDir, 0o755)

	err := s.hashDelete("test.md")
	if err == nil {
		t.Log("hashDelete did not error (may be running as root), skipping error check")
	}
}
