package store

import (
	"errors"
	"sync"
)

var ErrStoreFull = errors.New("global context store is full")

type Store interface {
	CheckCapacity(key string) error
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
	if maxEntries < 0 {
		maxEntries = 0
	}
	return &store{
		store:      make(map[string]Entry),
		maxEntries: maxEntries,
	}
}

func (l *store) CheckCapacity(key string) error {
	l.RLock()
	defer l.RUnlock()
	if _, exists := l.store[key]; !exists && l.maxEntries > 0 && len(l.store) >= l.maxEntries {
		return ErrStoreFull
	}
	return nil
}

func (l *store) Set(key string, val Entry) error {
	l.Lock()
	old, exists := l.store[key]
	if !exists && l.maxEntries > 0 && len(l.store) >= l.maxEntries {
		l.Unlock()
		return ErrStoreFull
	}
	l.store[key] = val
	l.Unlock()
	if old != nil {
		old.Stop()
	}
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
	entry := l.store[key]
	delete(l.store, key)
	l.Unlock()
	if entry != nil {
		entry.Stop()
	}
}
