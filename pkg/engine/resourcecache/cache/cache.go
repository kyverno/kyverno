package cache

import (
	"sync"

	"github.com/dgraph-io/ristretto"
)

type Cache interface {
	Add(key string, val ResourceEntry) bool
	Get(key string) (ResourceEntry, bool)
	Update(key string, val ResourceEntry) bool
	Delete(key string) bool
}

type ResourceEntry interface {
	Get() (interface{}, error)
	Stop()
}

type cache struct {
	sync.Mutex
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

func (l *cache) Add(key string, val ResourceEntry) bool {
	l.Lock()
	defer l.Unlock()
	return l.store.Set(key, val, 0)
}

func (l *cache) Get(key string) (ResourceEntry, bool) {
	l.Lock()
	defer l.Unlock()
	val, ok := l.store.Get(key)
	if !ok {
		return nil, ok
	}

	entry, ok := val.(ResourceEntry)
	return entry, ok
}

func (l *cache) Update(key string, val ResourceEntry) bool {
	l.Lock()
	defer l.Unlock()
	return l.store.Set(key, val, 0)
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
