package utils

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAcquireRelease(t *testing.T) {
	lock, err := Acquire(t.TempDir()+"/test.lock", true)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("Release failed: %v", err)
	}
}

func TestExclusiveLockSerializes(t *testing.T) {
	lockPath := t.TempDir() + "/test.lock"

	// Acquire exclusive lock in the current goroutine.
	lock1, err := Acquire(lockPath, true)
	if err != nil {
		t.Fatalf("Acquire lock1: %v", err)
	}

	var acquired int32
	done := make(chan struct{})

	// Second goroutine tries to acquire — should block until lock1 releases.
	go func() {
		lock2, err := Acquire(lockPath, true)
		if err != nil {
			t.Errorf("Acquire lock2: %v", err)
			close(done)
			return
		}
		atomic.AddInt32(&acquired, 1)
		_ = lock2.Release()
		close(done)
	}()

	// Give the goroutine time to block on flock.
	time.Sleep(100 * time.Millisecond)
	if atomic.LoadInt32(&acquired) != 0 {
		t.Fatal("second acquire should not have succeeded while lock is held")
	}

	// Release lock1 — unblocks lock2.
	if err := lock1.Release(); err != nil {
		t.Fatalf("Release lock1: %v", err)
	}

	<-done
	if atomic.LoadInt32(&acquired) != 1 {
		t.Fatal("second acquire should have succeeded after lock release")
	}
}

func TestSharedLocksCoexist(t *testing.T) {
	lockPath := t.TempDir() + "/test.lock"

	lock1, err := Acquire(lockPath, false)
	if err != nil {
		t.Fatalf("Acquire shared lock1: %v", err)
	}
	defer func() {
		if err := lock1.Release(); err != nil {
			t.Errorf("Release lock1: %v", err)
		}
	}()

	// Second shared lock should succeed immediately (no blocking).
	lock2, err := Acquire(lockPath, false)
	if err != nil {
		t.Fatalf("Acquire shared lock2: %v", err)
	}
	_ = lock2.Release()
}

func TestExclusiveBlocksShared(t *testing.T) {
	lockPath := t.TempDir() + "/test.lock"

	lock1, err := Acquire(lockPath, true)
	if err != nil {
		t.Fatalf("Acquire exclusive lock: %v", err)
	}

	var acquired int32
	done := make(chan struct{})

	go func() {
		lock2, err := Acquire(lockPath, false)
		if err != nil {
			t.Errorf("Acquire shared lock: %v", err)
			close(done)
			return
		}
		atomic.AddInt32(&acquired, 1)
		_ = lock2.Release()
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	if atomic.LoadInt32(&acquired) != 0 {
		t.Fatal("shared acquire should not succeed while exclusive lock is held")
	}

	_ = lock1.Release()
	<-done
	if atomic.LoadInt32(&acquired) != 1 {
		t.Fatal("shared acquire should succeed after exclusive release")
	}
}

func TestConcurrentExclusiveLocks(t *testing.T) {
	lockPath := t.TempDir() + "/test.lock"
	const goroutines = 10

	var counter int32
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			lock, err := Acquire(lockPath, true)
			if err != nil {
				t.Errorf("Acquire: %v", err)
				return
			}
			// Increment counter while holding lock — if flock works,
			// no two goroutines should overlap here.
			v := atomic.AddInt32(&counter, 1)
			if v != 1 {
				t.Errorf("counter should be 1 under exclusive lock, got %d", v)
			}
			time.Sleep(5 * time.Millisecond)
			atomic.AddInt32(&counter, -1)
			_ = lock.Release()
		}()
	}

	wg.Wait()
}
