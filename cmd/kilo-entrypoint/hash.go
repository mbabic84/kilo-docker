package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

// withFlock acquires a flock, runs fn, then releases it. Used to wrap hash
// operations that need cross-container coordination.
func (s *Syncer) withFlock(exclusive bool, fn func() error) error {
	lock, err := utils.Acquire(s.lockFile, exclusive)
	if err != nil {
		utils.LogWarn("[ainstruct-sync] flock (exclusive=%v) failed: %v\n", exclusive, err)
		return fn()
	}
	defer lock.Release() //nolint:errcheck // best-effort release
	return fn()
}

// hashGet retrieves the cached content hash for a relative file path.
// Returns empty string if no hash exists or the hash file is missing.
func (s *Syncer) hashGet(relPath string) string {
	var result string
	_ = s.withFlock(false, func() error {
		result = s.hashGetUnlocked(relPath)
		return nil
	})
	return result
}

// hashSet stores or updates the content hash for a relative file path.
// Appends a new entry if the path isn't already tracked.
func (s *Syncer) hashSet(relPath, hash string) error {
	return s.withFlock(true, func() error {
		return s.hashSetUnlocked(relPath, hash)
	})
}

// hashDelete removes the cached content hash for a relative file path.
// No-op if the path isn't tracked or the hash file doesn't exist.
func (s *Syncer) hashDelete(relPath string) error {
	return s.withFlock(true, func() error {
		return s.hashDeleteUnlocked(relPath)
	})
}

// hashGetUnlocked is the inner implementation of hashGet without flock.
func (s *Syncer) hashGetUnlocked(relPath string) string {
	s.hashMu.Lock()
	defer s.hashMu.Unlock()
	data, err := os.ReadFile(s.hashFile)
	if err != nil {
		return ""
	}
	prefix := relPath + "="
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, prefix) {
			return line[len(prefix):]
		}
	}
	return ""
}

// hashSetUnlocked is the inner implementation of hashSet without flock.
func (s *Syncer) hashSetUnlocked(relPath, hash string) error {
	s.hashMu.Lock()
	defer s.hashMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.hashFile), 0o700); err != nil {
		return fmt.Errorf("creating hash directory: %w", err)
	}
	var lines []string
	prefix := relPath + "="
	found := false
	if data, err := os.ReadFile(s.hashFile); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, prefix) {
				lines = append(lines, prefix+hash)
				found = true
			} else {
				lines = append(lines, line)
			}
		}
	}
	if !found {
		lines = append(lines, prefix+hash)
	}
	if err := os.WriteFile(s.hashFile, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		utils.LogWarn("[ainstruct-sync] Failed to write hash file %s: %v\n", s.hashFile, err)
		return fmt.Errorf("writing hash file: %w", err)
	}
	return nil
}

// hashDeleteUnlocked is the inner implementation of hashDelete without flock.
func (s *Syncer) hashDeleteUnlocked(relPath string) error {
	s.hashMu.Lock()
	defer s.hashMu.Unlock()
	data, err := os.ReadFile(s.hashFile)
	if err != nil {
		return nil
	}
	prefix := relPath + "="
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, prefix) {
			continue
		}
		lines = append(lines, line)
	}
	if err := os.WriteFile(s.hashFile, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		utils.LogWarn("[ainstruct-sync] Failed to write hash file %s: %v\n", s.hashFile, err)
		return fmt.Errorf("writing hash file: %w", err)
	}
	return nil
}

// computeLocalHash returns a hex-encoded SHA-256 hash of the given bytes.
func computeLocalHash(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

// localHashGet retrieves the locally cached SHA-256 hash for a relative path.
func (s *Syncer) localHashGet(relPath string) string {
	var result string
	_ = s.withFlock(false, func() error {
		result = localHashGetUnlocked(s.localHashFile, relPath)
		return nil
	})
	return result
}

// localHashSet stores the local SHA-256 hash for a relative path.
func (s *Syncer) localHashSet(relPath, hash string) error {
	return s.withFlock(true, func() error {
		return localHashSetUnlocked(s.localHashFile, relPath, hash)
	})
}

// localHashDelete removes the local SHA-256 hash for a relative path.
func (s *Syncer) localHashDelete(relPath string) error {
	return s.withFlock(true, func() error {
		return localHashDeleteUnlocked(s.localHashFile, relPath)
	})
}

func localHashGetUnlocked(filePath, relPath string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	prefix := relPath + "="
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, prefix) {
			return line[len(prefix):]
		}
	}
	return ""
}

func localHashSetUnlocked(filePath, relPath, hash string) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
		return fmt.Errorf("creating local hash directory: %w", err)
	}
	var lines []string
	prefix := relPath + "="
	found := false
	if data, err := os.ReadFile(filePath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, prefix) {
				lines = append(lines, prefix+hash)
				found = true
			} else {
				lines = append(lines, line)
			}
		}
	}
	if !found {
		lines = append(lines, prefix+hash)
	}
	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

func localHashDeleteUnlocked(filePath, relPath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	prefix := relPath + "="
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, prefix) {
			continue
		}
		lines = append(lines, line)
	}
	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}
