package cache

import (
	"context"
	"sync"

	"github.com/dgraph-io/ristretto"
)

type Cache interface {
	Add(key string, val *CacheEntry) bool
	Get(key string) (ResourceEntry, bool)
	Update(key string, val *CacheEntry) bool
	Delete(key string) bool
}

type ResourceEntry interface {
	Get() (interface{}, error)
}

type CacheEntry struct {
	Entry ResourceEntry
	Stop  context.CancelFunc
}

func (c *CacheEntry) Get() (interface{}, error) {
	return c.Entry.Get()
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

func (l *cache) Add(key string, val *CacheEntry) bool {
	l.Lock()
	defer l.Unlock()
	if val.Entry == nil {
		return false
	}
	return l.store.Set(key, val, 0)
}

func (l *cache) Get(key string) (ResourceEntry, bool) {
	l.Lock()
	defer l.Unlock()
	val, ok := l.store.Get(key)
	if !ok {
		return nil, ok
	}

	entry, ok := val.(*CacheEntry)
	return entry, ok
}

func (l *cache) Update(key string, val *CacheEntry) bool {
	l.Lock()
	defer l.Unlock()
	if val.Entry != nil {
		return false
	}
	return l.store.Set(key, val, 0)
}

func (l *cache) Delete(key string) bool {
	l.Lock()
	defer l.Unlock()

	l.store.Del(key)
	_, ok := l.store.Get(key)
	return ok
}

func ristrettoOnExit(val interface{}) {
	if entry, ok := val.(*CacheEntry); ok {
		entry.Stop()
	}
}
