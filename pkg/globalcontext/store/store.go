package store

import (
	"errors"
	"sync"
)

var ErrStoreFull = errors.New("global context store is full")

type Store interface {
	Set(key string, val Entry) error
	Get(key string) (Entry, bool)
	Delete(key string)
}

type store struct {
	sync.RWMutex
	store      map[string]Entry
	maxEntries int
}

func New(maxEntries int) Store {
	return &store{
		store:      make(map[string]Entry),
		maxEntries: maxEntries,
	}
}

func (l *store) Set(key string, val Entry) error {
	l.Lock()
	defer l.Unlock()
	old, exists := l.store[key]
	if !exists && l.maxEntries > 0 && len(l.store) >= l.maxEntries {
		return ErrStoreFull
	}
	// If the key already exists, stop it before replacing it
	if old != nil {
		old.Stop()
	}
	l.store[key] = val
	return nil
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
