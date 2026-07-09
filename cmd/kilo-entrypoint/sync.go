package main

import (
	"context"
	"os/signal"
	"syscall"
	"time"

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
		_ = s.cleanupDuplicates()
		s.pushUnsynced()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	const (
		minBackoff = 5 * time.Second
		maxBackoff = 5 * time.Minute
	)
	backoff := minBackoff

	for {
		if s.authExpired {
			utils.LogError("[ainstruct-sync] Auth expired permanently — stopping\n")
			return
		}

		err := runWatcher(ctx, s)
		if ctx.Err() != nil {
			utils.Log("[ainstruct-sync] Shutting down\n")
			return
		}
		if err != nil && err.Error() != "auth expired" {
			utils.LogError("[ainstruct-sync] Watcher exited: %v — restarting in %s\n", err, backoff)
		} else if err != nil {
			utils.LogError("[ainstruct-sync] Watcher exited: auth expired — restarting in %s\n", backoff)
		} else {
			utils.Log("[ainstruct-sync] Watcher exited unexpectedly — restarting in %s\n", backoff)
		}

		select {
		case <-ctx.Done():
			utils.Log("[ainstruct-sync] Shutting down\n")
			return
		case <-time.After(backoff):
		}

		if s.authExpired {
			utils.LogError("[ainstruct-sync] Auth expired permanently — stopping\n")
			return
		}

		backoff = backoff * 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}

		if err := s.pullCollection(); err != nil {
			utils.LogError("[ainstruct-sync] Pull on restart failed: %v\n", err)
		} else {
			backoff = minBackoff
		}
		s.pushUnsynced()
	}
}
