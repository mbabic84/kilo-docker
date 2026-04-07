package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

// hashGet retrieves the cached content hash for a relative file path.
// Returns empty string if no hash exists or the hash file is missing.
func (s *Syncer) hashGet(relPath string) string {
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

// hashSet stores or updates the content hash for a relative file path.
// Appends a new entry if the path isn't already tracked.
func (s *Syncer) hashSet(relPath, hash string) error {
	s.hashMu.Lock()
	defer s.hashMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.hashFile), 0o755); err != nil {
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
	if err := os.WriteFile(s.hashFile, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		utils.LogWarn("[ainstruct-sync] Failed to write hash file %s: %v\n", s.hashFile, err)
		return fmt.Errorf("writing hash file: %w", err)
	}
	return nil
}

// hashDelete removes the cached content hash for a relative file path.
// No-op if the path isn't tracked or the hash file doesn't exist.
func (s *Syncer) hashDelete(relPath string) error {
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
	if err := os.WriteFile(s.hashFile, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		utils.LogWarn("[ainstruct-sync] Failed to write hash file %s: %v\n", s.hashFile, err)
		return fmt.Errorf("writing hash file: %w", err)
	}
	return nil
}
