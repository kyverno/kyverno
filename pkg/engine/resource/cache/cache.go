package cache

import (
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

type Cache interface {
	Set(key string, val ResourceEntry) bool
	Get(key string) (ResourceEntry, bool)
	Delete(key string)
}

type ResourceEntry interface {
	Get() (interface{}, error)
	LastUpdated() time.Time
	Stop()
}

type invalidentry struct {
	err error
}

func (i *invalidentry) Get() (interface{}, error) {
	return nil, errors.Wrapf(i.err, "failed to create cached context entry")
}

func (i *invalidentry) LastUpdated() time.Time {
	return time.Time{}
}

func (i *invalidentry) Stop() {}

func NewInvalidEntry(err error) ResourceEntry {
	return &invalidentry{
		err: err,
	}
}

type cache struct {
	sync.RWMutex
	store map[string]ResourceEntry
}

func New() Cache {
	return &cache{
		store: make(map[string]ResourceEntry),
	}
}

func (l *cache) Set(key string, val ResourceEntry) bool {
	l.Lock()
	defer l.Unlock()

	l.store[key] = val
	_, ok := l.store[key]
	return ok
}

func (l *cache) Get(prefix string) (ResourceEntry, bool) {
	l.RLock()
	defer l.RUnlock()

	t := time.Time{}
	var entry ResourceEntry = nil
	for k, v := range l.store {
		if !strings.HasPrefix(k, prefix) {
			continue
		}

		if v.LastUpdated().After(t) {
			entry = v
		}
	}
	if entry == nil {
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
