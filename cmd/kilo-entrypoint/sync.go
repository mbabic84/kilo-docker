package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/kilo-org/kilo-docker/pkg/constants"
)

// runSyncMode starts the ainstruct-sync background process. It pulls the
// remote collection to sync local files, then starts an inotify-based file
// watcher that uploads changes to the Ainstruct REST API. All output goes
// to a log file (~/.config/kilo/ainstruct-sync.log) to avoid interfering
// with the Kilo TUI on stdout.
func runSyncMode() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("")

	home := constants.GetHomeDir()
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
