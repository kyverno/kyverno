package cache

import (
	"context"
	"sync"

	"github.com/dgraph-io/ristretto"
)

type CacheEntry interface {
	Get() ([]byte, error)
}

type cacheEntry struct {
	entry CacheEntry
	stop  context.CancelFunc
}

func (c *cacheEntry) Get() ([]byte, error) {
	return c.entry.Get()
}

type Cache struct {
	sync.Mutex
	store *ristretto.Cache
}

func New() (*Cache, error) {
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

	return &Cache{
		store: rcache,
	}, nil
}

func (l *Cache) Add(key string, val *cacheEntry) bool {
	l.Lock()
	defer l.Unlock()
	if val.entry != nil {
		return false
	}
	return l.store.Set(key, val, 0)
}

func (l *Cache) Get(key string) (CacheEntry, bool) {
	l.Lock()
	defer l.Unlock()
	val, ok := l.store.Get(key)
	if !ok {
		return nil, ok
	}

	entry, ok := val.(*cacheEntry)
	return entry, ok
}

func (l *Cache) Update(key string, val *cacheEntry) bool {
	l.Lock()
	defer l.Unlock()
	if val.entry != nil {
		return false
	}
	return l.store.Set(key, val, 0)
}

func (l *Cache) Delete(key string) bool {
	l.Lock()
	defer l.Unlock()

	l.store.Del(key)
	_, ok := l.store.Get(key)
	return ok
}

func ristrettoOnExit(val interface{}) {
	if entry, ok := val.(*cacheEntry); ok {
		entry.stop()
	}
}
