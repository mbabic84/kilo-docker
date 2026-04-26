package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mbabic84/kilo-docker/pkg/utils"
	"golang.org/x/sys/unix"
)

// pendingEvent tracks a debounced file system event for sync processing.
type pendingEvent struct {
	eventType   string
	lastEventAt time.Time
}

// inotifyEvent represents a raw inotify event parsed from the kernel buffer.
type inotifyEvent struct {
	wd      int32
	mask    uint32
	cookie  uint32
	nameLen uint32
	name    string
}

// parseInotifyEvents parses a raw inotify event buffer into a slice of
// inotifyEvent structs, handling 4-byte alignment between events.
func parseInotifyEvents(buf []byte, n int) []inotifyEvent {
	var events []inotifyEvent
	offset := 0
	for offset < n {
		if offset+16 > n {
			break
		}
		header := buf[offset : offset+16]
		wd := int32(binary.LittleEndian.Uint32(header[0:4]))
		mask := binary.LittleEndian.Uint32(header[4:8])
		cookie := binary.LittleEndian.Uint32(header[8:12])
		nameLen := binary.LittleEndian.Uint32(header[12:16])
		nameBytesEnd := offset + 16 + int(nameLen)
		if nameBytesEnd > n {
			break
		}
		name := ""
		if nameLen > 0 {
			nameBytes := buf[offset+16 : nameBytesEnd]
			name = string(bytes.TrimRight(nameBytes, "\x00"))
		}
		events = append(events, inotifyEvent{wd: wd, mask: mask, cookie: cookie, nameLen: nameLen, name: name})
		offset = nameBytesEnd
		for offset%4 != 0 {
			offset++
		}
	}
	return events
}

const debounceInterval = 5 * time.Second

// runWatcher starts an inotify-based file watcher on the Kilo config
// directories. It detects CREATE, MODIFY, DELETE, and MOVED events,
// debounces them for 5 seconds to avoid syncing intermediate states,
// and triggers syncFile or deleteByPath on the Syncer.
// shouldWatchDir returns true if the directory should be watched.
// Excludes hidden dirs, node_modules, .git, and log files.
func shouldWatchDir(path string) bool {
	base := filepath.Base(path)
	// Skip hidden directories
	if strings.HasPrefix(base, ".") {
		return false
	}
	// Skip common non-sync directories
	switch base {
	case "node_modules", "vendor", "__pycache__", "dist", "build":
		return false
	}
	return true
}

func collectWatchDirs(watchDirs, watchFiles []string) []string {
	dirs := make([]string, 0, len(watchDirs)+len(watchFiles))
	seen := make(map[string]struct{}, len(watchDirs)+len(watchFiles))
	add := func(dir string) {
		if dir == "" {
			return
		}
		if _, ok := seen[dir]; ok {
			return
		}
		seen[dir] = struct{}{}
		dirs = append(dirs, dir)
	}

	for _, dir := range watchDirs {
		add(dir)
	}
	for _, file := range watchFiles {
		add(filepath.Dir(file))
	}
	return dirs
}

func runWatcher(ctx context.Context, s *Syncer) error {
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC)
	if err != nil {
		return fmt.Errorf("inotify_init1: %w", err)
	}
	defer func() { _ = syscall.Close(fd) }()

	watchDirs := s.syncedAbsDirs()
	watchFiles := s.syncedAbsFiles()
	watchRoots := collectWatchDirs(watchDirs, watchFiles)
	for _, dir := range watchRoots {
		_ = os.MkdirAll(dir, 0o700)
	}

	const watchMask = unix.IN_CREATE | unix.IN_MODIFY | unix.IN_DELETE | unix.IN_MOVED_TO | unix.IN_MOVED_FROM

	// wdToPath maps watch descriptors to their paths.
	// For directories, this is the directory path.
	// For files, this is the full file path.
	wdToPath := make(map[int32]string)

	watchedDirs := make(map[string]struct{})
	addDirWatch := func(dir string) {
		if _, ok := watchedDirs[dir]; ok {
			return
		}
		wd, err := unix.InotifyAddWatch(fd, dir, watchMask)
		if err != nil {
			utils.Log("[ainstruct-sync] Failed to watch dir %s: %v\n", dir, err)
			return
		}
		wd32, ok := safeInt32(wd)
		if !ok {
			utils.LogWarn("[ainstruct-sync] Watch descriptor overflow for dir %s\n", dir)
			return
		}
		wdToPath[wd32] = dir
		watchedDirs[dir] = struct{}{}
	}

	addFileWatch := func(file string) {
		wd, err := unix.InotifyAddWatch(fd, file, watchMask)
		if err != nil {
			utils.Log("[ainstruct-sync] Failed to watch file %s: %v\n", file, err)
			return
		}
		wd32, ok := safeInt32(wd)
		if !ok {
			utils.LogWarn("[ainstruct-sync] Watch descriptor overflow for file %s\n", file)
			return
		}
		wdToPath[wd32] = file
	}

	// Watch directories recursively
	for _, dir := range watchRoots {
		addDirWatch(dir)
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() && path != dir && shouldWatchDir(path) {
				addDirWatch(path)
			}
			return nil
		})
	}

	// Watch individual files (like opencode.json)
	for _, file := range watchFiles {
		addFileWatch(file)
	}

	utils.Log("[ainstruct-sync] Watcher started, watching %d paths\n", len(wdToPath))
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

		pollFD, ok := safeInt32(fd)
		if !ok {
			return fmt.Errorf("poll fd overflow: %d", fd)
		}
		fds := []unix.PollFd{{Fd: pollFD, Events: unix.POLLIN}}
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
			watchedPath := wdToPath[ev.wd]
			if watchedPath == "" {
				continue
			}

			// Determine full path:
			// - For directory watches: watchedPath is dir, ev.name is filename
			// - For file watches: watchedPath is full file path, ev.name is empty
			var fullPath string
			if ev.name != "" {
				fullPath = filepath.Join(watchedPath, ev.name)
			} else {
				fullPath = watchedPath
			}

			// Only sync files that are whitelisted via syncPaths.
			relPath := strings.TrimPrefix(fullPath, s.kiloConfigDir+"/")
			if !s.isSyncedPath(relPath) {
				continue
			}

			if ev.mask&unix.IN_ISDIR != 0 {
				// Directory events (create/modify/delete/move) are not syncable
				// files. For new directories under any watched dir, add a watch
				// so we track future file changes inside them, then skip the event.
				if ev.mask&unix.IN_CREATE != 0 && shouldWatchDir(fullPath) {
					addDirWatch(fullPath)
				}
				continue
			}

			var eventType string
			if ev.mask&(unix.IN_DELETE|unix.IN_MOVED_FROM) != 0 {
				eventType = "DELETE"
			} else {
				eventType = "MODIFY"
			}

			utils.Log("[ainstruct-sync] Queueing %s event for: %s\n", eventType, relPath)
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

func safeInt32(v int) (int32, bool) {
	if v < math.MinInt32 || v > math.MaxInt32 {
		return 0, false
	}
	return int32(v), true
}

// debounceLoop runs in a background goroutine, processing pending events
// after their debounce interval (5 seconds) has elapsed without new changes.
// This prevents syncing intermediate file states during rapid saves.
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

		deadlineCopy := deadline
		mu.Unlock()

		timer := time.NewTimer(time.Until(deadlineCopy))
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

// processFile handles a single debounced file event: syncs the file to the
// remote collection if it was modified, or deletes it remotely if it was removed.
func processFile(s *Syncer, fullPath, eventType string) {
	relPath := strings.TrimPrefix(fullPath, s.kiloConfigDir+"/")
	utils.Log("[ainstruct-sync] Processing %s event for: %s\n", eventType, relPath)
	if eventType == "DELETE" {
		if err := s.deleteByPath(relPath); err != nil && !s.authExpired {
			utils.LogError("[ainstruct-sync] Delete error for %s: %v\n", relPath, err)
		}
	} else {
		if _, err := os.Stat(fullPath); err == nil {
			if err := s.syncFile(fullPath); err != nil && !s.authExpired {
				utils.LogError("[ainstruct-sync] Sync error for %s: %v\n", relPath, err)
			}
		} else {
			utils.Log("[ainstruct-sync] File not found, skipping: %s\n", relPath)
		}
	}
}
