package cache

import (
	"sync"

	"github.com/dgraph-io/ristretto"
	"github.com/pkg/errors"
)

type Cache interface {
	Set(key string, val ResourceEntry) bool
	Get(key string) (ResourceEntry, bool)
	Delete(key string) bool
}

type ResourceEntry interface {
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

func NewInvalidEntry(err error) ResourceEntry {
	return &invalidentry{
		err: err,
	}
}

type cache struct {
	sync.RWMutex
	store *ristretto.Cache
}

func New() (Cache, error) {
	config := ristretto.Config{
		MaxCost:     100 * 1000 * 1000, // 100 MB
		NumCounters: 10 * 100,          // 100 entries
		BufferItems: 64,
		OnExit:      ristrettoOnExit,
	}

	rcache, err := ristretto.NewCache(&config)
	if err != nil {
		return nil, err
	}

	return &cache{
		store: rcache,
	}, nil
}

func (l *cache) Set(key string, val ResourceEntry) bool {
	l.Lock()
	defer l.Unlock()
	return l.store.Set(key, val, 0)
}

func (l *cache) Get(key string) (ResourceEntry, bool) {
	l.RLock()
	defer l.RUnlock()
	val, ok := l.store.Get(key)
	if !ok {
		return nil, ok
	}

	entry, ok := val.(ResourceEntry)
	if !ok {
		return nil, ok
	}
	return entry, ok
}

func (l *cache) Delete(key string) bool {
	l.Lock()
	defer l.Unlock()

	l.store.Del(key)
	_, ok := l.store.Get(key)
	return !ok
}

func ristrettoOnExit(val interface{}) {
	if entry, ok := val.(ResourceEntry); ok {
		entry.Stop()
	}
}
