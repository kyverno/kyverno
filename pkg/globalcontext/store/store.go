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
	store      map[string]Entry
	maxEntries int
}

func New(maxEntries int) Store {
	return &store{
		store:      make(map[string]Entry),
		maxEntries: maxEntries,
	}
}

func (l *store) Set(key string, val Entry) {
	l.Lock()
	defer l.Unlock()
	old := l.store[key]
	if old != nil {
		old.Stop()
	} else if l.maxEntries > 0 && len(l.store) >= l.maxEntries {
		if val != nil {
			val.Stop()
		}
		return
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
