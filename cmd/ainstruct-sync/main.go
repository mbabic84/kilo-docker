package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("")

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
