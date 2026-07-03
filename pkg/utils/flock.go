package utils

import (
	"os"
	"path/filepath"
	"syscall" //nolint:depguard // syscall.Flock is the only way to do cross-process flock
)

// FileLock wraps a file descriptor used with flock(2) for cross-process
// coordination. The kernel automatically releases the lock if the process
// crashes, preventing deadlocks.
type FileLock struct {
	file *os.File
}

// Acquire opens (or creates) the lock file at path and acquires a flock.
// If exclusive is true, the lock is exclusive (LOCK_EX); otherwise it is
// shared (LOCK_SH). The caller must call Release when done.
func Acquire(path string, exclusive bool) (*FileLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { //nolint:gosec // fixed permission
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // lock file
	if err != nil {
		return nil, err
	}
	flag := syscall.LOCK_SH
	if exclusive {
		flag = syscall.LOCK_EX
	}
	if err := syscall.Flock(int(f.Fd()), flag); err != nil { //nolint:gosec // fd is small
		if closeErr := f.Close(); closeErr != nil {
			return nil, closeErr
		}
		return nil, err
	}
	return &FileLock{file: f}, nil
}

// Release unlocks the flock and closes the underlying file descriptor.
func (l *FileLock) Release() error {
	err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN) //nolint:gosec // fd is small
	if closeErr := l.file.Close(); closeErr != nil && err == nil {
		return closeErr
	}
	return err
}
