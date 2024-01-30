package store

import (
	"sync"

	"github.com/pkg/errors"
)

type Store interface {
	Set(key string, val Entry) bool
	Get(key string) (Entry, bool)
	Delete(key string)
}

type Entry interface {
	Get() (interface{}, error)
	Stop()
}

type invalidentry struct {
	err error
}

func (i *invalidentry) Get() (interface{}, error) {
	return nil, errors.Wrapf(i.err, "failed to create cached context entry")
}

func (i *invalidentry) Stop() {}

func NewInvalidEntry(err error) Entry {
	return &invalidentry{
		err: err,
	}
}

type cache struct {
	sync.RWMutex
	store map[string]Entry
}

func New() Store {
	return &cache{
		store: make(map[string]Entry),
	}
}

func (l *cache) Set(key string, val Entry) bool {
	l.Lock()
	defer l.Unlock()

	if val, found := l.store[key]; found { // If the key already exists, skip it before replacing it
		val.Stop()
	}

	l.store[key] = val
	_, ok := l.store[key]
	return ok
}

func (l *cache) Get(key string) (Entry, bool) {
	l.RLock()
	defer l.RUnlock()

	entry, ok := l.store[key]
	if !ok {
		return nil, false
	}
	return entry, true
}

func (l *cache) Delete(key string) {
	l.Lock()
	defer l.Unlock()

	val, ok := l.store[key]
	if !ok {
		return // value already deleted
	}
	val.Stop()
	delete(l.store, key)
}
