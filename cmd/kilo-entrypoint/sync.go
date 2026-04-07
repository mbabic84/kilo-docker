package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/mbabic84/kilo-docker/pkg/utils"
)

// runSyncMode starts the ainstruct-sync background process. It pulls the
// remote collection to sync local files, then starts an inotify-based file
// watcher that uploads changes to the Ainstruct REST API. All output goes
// to a log file (~/.config/kilo/ainstruct-sync.log) to avoid interfering
// with the Kilo TUI on stdout.
func runSyncMode() {
	s := NewSyncer()

	if err := s.pullCollection(); err != nil {
		utils.LogError("[ainstruct-sync] Pull failed: %v\n", err)
	} else {
		s.pushUnsynced()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	if err := runWatcher(ctx, s); err != nil {
		utils.LogError("[ainstruct-sync] Watcher error: %v\n", err)
		os.Exit(1)
	}
	utils.Log("[ainstruct-sync] Shutting down\n")
}
