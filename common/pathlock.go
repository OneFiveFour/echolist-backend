package common

import (
	"sort"
	"sync"
)

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

// LockMany acquires exclusive locks for the provided paths in sorted order.
// Sorting keeps lock acquisition deterministic when an operation touches
// multiple paths at once, such as a file rename from old title -> new title.
// Duplicate paths are ignored. Callers must invoke the returned unlock function.
func (l *Locker) LockMany(paths ...string) (unlock func()) {
	seen := make(map[string]struct{}, len(paths))
	unique := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		unique = append(unique, path)
	}

	sort.Strings(unique)

	unlocks := make([]func(), 0, len(unique))
	for _, path := range unique {
		unlocks = append(unlocks, l.Lock(path))
	}

	return func() {
		for i := len(unlocks) - 1; i >= 0; i-- {
			unlocks[i]()
		}
	}
}
