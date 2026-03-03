package common

import "sync"

// Locker provides per-path mutual exclusion. The zero value is ready to use.
type Locker struct {
	mu    sync.Mutex
	locks map[string]*entry
}

type entry struct {
	mu      sync.Mutex
	waiters int
}

// Lock acquires an exclusive lock for the given path.
// Callers must call the returned unlock function when done.
func (l *Locker) Lock(path string) (unlock func()) {
	l.mu.Lock()
	if l.locks == nil {
		l.locks = make(map[string]*entry)
	}
	e, ok := l.locks[path]
	if !ok {
		e = &entry{}
		l.locks[path] = e
	}
	e.waiters++
	l.mu.Unlock()

	e.mu.Lock()

	return func() {
		e.mu.Unlock()

		l.mu.Lock()
		e.waiters--
		if e.waiters == 0 {
			delete(l.locks, path)
		}
		l.mu.Unlock()
	}
}
