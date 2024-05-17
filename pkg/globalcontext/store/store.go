package store

import (
	"sync"
)

type Store interface {
	Set(key string, val Entry)
	Get(key string) (Entry, bool)
	Delete(key string)
}

type store struct {
	sync.RWMutex
	store map[string]Entry
}

func New() Store {
	return &store{
		store: make(map[string]Entry),
	}
}

func (l *store) Set(key string, val Entry) {
	l.Lock()
	defer l.Unlock()
	old := l.store[key]
	// If the key already exists, stop it before replacing it
	if old != nil {
		old.Stop()
	}
	l.store[key] = val
}

func (l *store) Get(key string) (Entry, bool) {
	l.RLock()
	defer l.RUnlock()
	entry, ok := l.store[key]
	return entry, ok
}

func (l *store) Delete(key string) {
	l.Lock()
	defer l.Unlock()
	entry := l.store[key]
	if entry != nil {
		entry.Stop()
	}
	delete(l.store, key)
}
