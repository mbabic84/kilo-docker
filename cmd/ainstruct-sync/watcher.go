package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type pendingEvent struct {
	eventType   string
	lastEventAt time.Time
}

type inotifyEvent struct {
	wd      int32
	mask    uint32
	cookie  uint32
	nameLen uint32
	name    string
}

func parseInotifyEvents(buf []byte, n int) []inotifyEvent {
	var events []inotifyEvent
	offset := 0
	for offset < n {
		if offset+16 > n {
			break
		}
		wd := int32(buf[offset]) | int32(buf[offset+1])<<8 | int32(buf[offset+2])<<16 | int32(buf[offset+3])<<24
		mask := uint32(buf[offset+4]) | uint32(buf[offset+5])<<8 | uint32(buf[offset+6])<<16 | uint32(buf[offset+7])<<24
		cookie := uint32(buf[offset+8]) | uint32(buf[offset+9])<<8 | uint32(buf[offset+10])<<16 | uint32(buf[offset+11])<<24
		nameLen := uint32(buf[offset+12]) | uint32(buf[offset+13])<<8 | uint32(buf[offset+14])<<16 | uint32(buf[offset+15])<<24
		name := ""
		if nameLen > 0 {
			nameBytes := buf[offset+16 : offset+16+int(nameLen)]
			name = string(bytes.TrimRight(nameBytes, "\x00"))
		}
		events = append(events, inotifyEvent{wd: wd, mask: mask, cookie: cookie, nameLen: nameLen, name: name})
		offset += 16 + int(nameLen)
		for offset%4 != 0 {
			offset++
		}
	}
	return events
}

const debounceInterval = 5 * time.Second

func runWatcher(ctx context.Context, s *Syncer) error {
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC)
	if err != nil {
		return fmt.Errorf("inotify_init1: %w", err)
	}
	defer syscall.Close(fd)

	watchDirs := []string{
		filepath.Join(s.homeDir, ".config", "kilo", "rules"),
		filepath.Join(s.homeDir, ".config", "kilo", "commands"),
		filepath.Join(s.homeDir, ".config", "kilo", "agents"),
		filepath.Join(s.homeDir, ".config", "kilo", "plugins"),
		filepath.Join(s.homeDir, ".config", "kilo", "skills"),
		filepath.Join(s.homeDir, ".config", "kilo", "tools"),
	}
	for _, dir := range watchDirs {
		os.MkdirAll(dir, 0o755)
	}

	const watchMask = unix.IN_CREATE | unix.IN_MODIFY | unix.IN_DELETE | unix.IN_MOVED_TO | unix.IN_MOVED_FROM

	wdToDir := make(map[int32]string)
	addWatch := func(dir string) {
		wd, err := unix.InotifyAddWatch(fd, dir, watchMask)
		if err != nil {
			log.Printf("[ainstruct-sync] Failed to watch %s: %v", dir, err)
			return
		}
		wdToDir[int32(wd)] = dir
	}

	// Add watches on flat directories directly
	for _, dir := range watchDirs[:4] { // rules, commands, agents, plugins
		addWatch(dir)
	}

	// Skills uses nested directories (skills/<name>/SKILL.md) — watch recursively
	skillsDir := watchDirs[4]
	addWatch(skillsDir)
	filepath.Walk(skillsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && path != skillsDir {
			addWatch(path)
		}
		return nil
	})

	// Tools directory — watch directly
	addWatch(watchDirs[5])
	opencodePath := filepath.Join(s.homeDir, ".config", "kilo", "opencode.json")
	if _, err := os.Stat(opencodePath); err == nil {
		wd, err := unix.InotifyAddWatch(fd, opencodePath, unix.IN_MODIFY|unix.IN_DELETE|unix.IN_MOVED_TO|unix.IN_MOVED_FROM)
		if err != nil {
			log.Printf("[ainstruct-sync] Failed to watch opencode.json: %v", err)
		} else {
			wdToDir[int32(wd)] = filepath.Dir(opencodePath)
		}
	}

	log.Println("[ainstruct-sync] Watcher started")
	buf := make([]byte, 4096)
	pending := make(map[string]*pendingEvent)
	var pendingMu sync.Mutex
	wake := make(chan struct{}, 1)

	go debounceLoop(ctx, s, pending, &pendingMu, wake)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		_, err := unix.Poll(fds, -1)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return fmt.Errorf("poll: %w", err)
		}

		n, err := syscall.Read(fd, buf)
		if err != nil {
			return fmt.Errorf("inotify read: %w", err)
		}
		if n == 0 {
			continue
		}

		events := parseInotifyEvents(buf[:n], n)
		pendingMu.Lock()
		for _, ev := range events {
			dir := wdToDir[ev.wd]
			if dir == "" {
				continue
			}
			var fullPath string
			if dir == filepath.Join(s.homeDir, ".config", "kilo") && ev.name == "opencode.json" {
				fullPath = opencodePath
			} else {
				fullPath = filepath.Join(dir, ev.name)
			}

			// Dynamically watch new subdirectories under skills/
			if ev.mask&unix.IN_ISDIR != 0 && ev.mask&unix.IN_CREATE != 0 && strings.HasPrefix(dir, skillsDir) {
				addWatch(fullPath)
			}

			var eventType string
			if ev.mask&(unix.IN_DELETE|unix.IN_MOVED_FROM) != 0 {
				eventType = "DELETE"
			} else {
				eventType = "MODIFY"
			}

			if p, ok := pending[fullPath]; ok {
				p.lastEventAt = time.Now()
				p.eventType = eventType
			} else {
				pending[fullPath] = &pendingEvent{eventType: eventType, lastEventAt: time.Now()}
			}
		}
		pendingMu.Unlock()

		select {
		case wake <- struct{}{}:
		default:
		}
	}
}

// debounceLoop runs in a goroutine. Each iteration:
// 1. Find the file with the earliest lastEventAt — its deadline is lastEventAt + 5s
// 2. Sleep until that deadline (or until a wake signal arrives)
// 3. Sync all files whose 5s window has elapsed
// 4. Loop
func debounceLoop(ctx context.Context, s *Syncer, pending map[string]*pendingEvent, mu *sync.Mutex, wake <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		mu.Lock()
		var earliest time.Time
		for _, p := range pending {
			if earliest.IsZero() || p.lastEventAt.Before(earliest) {
				earliest = p.lastEventAt
			}
		}

		if earliest.IsZero() {
			mu.Unlock()
			select {
			case <-ctx.Done():
				return
			case <-wake:
				continue
			}
		}

		deadline := earliest.Add(debounceInterval)
		now := time.Now()

		if now.After(deadline) || now.Equal(deadline) {
			for path, p := range pending {
				if time.Since(p.lastEventAt) >= debounceInterval {
					processFile(s, path, p.eventType)
					delete(pending, path)
				}
			}
			mu.Unlock()
			continue
		}

		// Copy pending snapshot before sleeping (unlock early)
		deadlineCopy := deadline
		mu.Unlock()

		timer := time.NewTimer(deadlineCopy.Sub(time.Now()))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-wake:
			timer.Stop()
			continue
		case <-timer.C:
			continue
		}
	}
}

func processFile(s *Syncer, fullPath, eventType string) {
	relPath := strings.TrimPrefix(fullPath, s.homeDir+"/")
	if eventType == "DELETE" {
		if err := s.deleteByPath(relPath); err != nil && !s.authExpired {
			log.Printf("[ainstruct-sync] Delete error for %s: %v", relPath, err)
		}
	} else {
		if _, err := os.Stat(fullPath); err == nil {
			if err := s.syncFile(fullPath); err != nil && !s.authExpired {
				log.Printf("[ainstruct-sync] Sync error for %s: %v", relPath, err)
			}
		}
	}
}
