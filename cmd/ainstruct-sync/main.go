package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("")

	// Set up file-based logging (no stdout — would interfere with Kilo TUI)
	home := os.Getenv("HOME")
	logDir := filepath.Join(home, ".config", "kilo")
	os.MkdirAll(logDir, 0o755)
	logPath := filepath.Join(logDir, "ainstruct-sync.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("[ainstruct-sync] Failed to open log file %s: %v", logPath, err)
	} else {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	s := NewSyncer()

	if err := s.pullCollection(); err != nil {
		log.Printf("[ainstruct-sync] Pull failed: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	if err := runWatcher(ctx, s); err != nil {
		log.Printf("[ainstruct-sync] Watcher error: %v", err)
		os.Exit(1)
	}
	log.Println("[ainstruct-sync] Shutting down")
}
