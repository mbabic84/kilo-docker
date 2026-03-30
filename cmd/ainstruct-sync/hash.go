package main

import (
	"os"
	"path/filepath"
	"strings"
)

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

func (s *Syncer) hashSet(relPath, hash string) {
	s.hashMu.Lock()
	defer s.hashMu.Unlock()
	os.MkdirAll(filepath.Dir(s.hashFile), 0o755)
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
	os.WriteFile(s.hashFile, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func (s *Syncer) hashDelete(relPath string) {
	s.hashMu.Lock()
	defer s.hashMu.Unlock()
	data, err := os.ReadFile(s.hashFile)
	if err != nil {
		return
	}
	prefix := relPath + "="
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, prefix) {
			continue
		}
		lines = append(lines, line)
	}
	os.WriteFile(s.hashFile, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
