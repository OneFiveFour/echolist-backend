package common

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestLocker_SerializesSamePath(t *testing.T) {
	var l Locker
	var counter int64
	var maxConcurrent int64
	var wg sync.WaitGroup

	const goroutines = 50

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := l.Lock("/same/path")
			defer unlock()

			cur := atomic.AddInt64(&counter, 1)
			if cur > 1 {
				atomic.StoreInt64(&maxConcurrent, cur)
			}
			atomic.AddInt64(&counter, -1)
		}()
	}

	wg.Wait()

	if maxConcurrent > 1 {
		t.Errorf("expected max concurrency 1 for same path, got %d", maxConcurrent)
	}
}

func TestLocker_AllowsDifferentPaths(t *testing.T) {
	var l Locker
	ready := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		unlock := l.Lock("/path/a")
		close(ready)
		<-make(chan struct{})
		unlock()
	}()

	<-ready

	done := make(chan struct{})
	go func() {
		unlock := l.Lock("/path/b")
		unlock()
		close(done)
	}()

	select {
	case <-done:
		// success
	}
}

func TestLocker_CleansUpEntries(t *testing.T) {
	var l Locker

	unlock := l.Lock("/tmp/file")
	unlock()

	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.locks) != 0 {
		t.Errorf("expected empty lock map after unlock, got %d entries", len(l.locks))
	}
}
