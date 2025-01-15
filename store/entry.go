package store

import "sync"

// TODO: use generics
type Entry struct {
	entry *Camera
	mu    sync.RWMutex
}

func (e *Entry) Read(fn func(*Camera)) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	fn(e.entry)
}

func (e *Entry) Write(fn func(*Camera)) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	fn(e.entry)
}
