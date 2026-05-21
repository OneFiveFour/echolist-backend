package common

import (
	"sort"
	"sync"
)

// Locker provides per-path mutual exclusion. The zero value is ready to use.
type Locker struct {
	mutex sync.Mutex
	locks map[string]*entry
}

type entry struct {
	mutex   sync.Mutex
	waiters int
}

// Lock acquires an exclusive lock for the given path.
// Callers must call the returned unlock function when done.
func (locker *Locker) Lock(path string) (unlock func()) {
	locker.mutex.Lock()
	if locker.locks == nil {
		locker.locks = make(map[string]*entry)
	}
	ent, ok := locker.locks[path]
	if !ok {
		ent = &entry{}
		locker.locks[path] = ent
	}
	ent.waiters++
	locker.mutex.Unlock()

	ent.mutex.Lock()

	return func() {
		ent.mutex.Unlock()

		locker.mutex.Lock()
		ent.waiters--
		if ent.waiters == 0 {
			delete(locker.locks, path)
		}
		locker.mutex.Unlock()
	}
}

// LockMany acquires exclusive locks for the provided paths in sorted order.
// Sorting keeps lock acquisition deterministic when an operation touches
// multiple paths at once, such as a file rename from old title -> new title.
// Duplicate paths are ignored. Callers must invoke the returned unlock function.
func (locker *Locker) LockMany(paths ...string) (unlock func()) {
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
		unlocks = append(unlocks, locker.Lock(path))
	}

	return func() {
		for i := len(unlocks) - 1; i >= 0; i-- {
			unlocks[i]()
		}
	}
}
