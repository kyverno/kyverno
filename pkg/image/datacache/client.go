// Package datacache caches the image metadata served to CEL policies through
// image.GetMetadata, across admission requests.
//
// Entries are keyed by the image string exactly as the policy wrote it, so the
// reference-specific fields (image, tag, resolvedImage, ...) are always the ones
// of that reference. A digest reference is immutable and lives for the TTL; a
// tag reference lives for the (usually much shorter) tag TTL, because a registry
// may move a tag at any time.
package datacache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/kyverno/kyverno/pkg/metrics"
	"golang.org/x/sync/singleflight"
)

const (
	// DefaultTTL is the digest TTL the flags default to.
	DefaultTTL = 60 * time.Minute
	// DefaultTagTTL is the tag TTL the flags default to.
	DefaultTagTTL = 60 * time.Second
	// DefaultMaxSize is the default cache budget in bytes (10 MiB).
	DefaultMaxSize = 10 << 20
	// expectedEntryCost is the assumed entry size in bytes, only used to size
	// ristretto's counters (10 per expected entry); the real capacity is MaxCost.
	// Underestimating the size only allocates more counters, which is harmless.
	expectedEntryCost = 8 << 10
	// fetchTimeout bounds a shared fetch, so a hung registry cannot keep an image in flight forever.
	fetchTimeout = 30 * time.Second
)

// Client caches image metadata. A nil Client means caching is disabled.
type Client interface {
	// Get returns the metadata of image, calling fetch on a miss. Errors are
	// never cached. The returned map is shared between callers and must not be
	// modified.
	Get(ctx context.Context, image string, fetch func(context.Context) (map[string]any, error)) (map[string]any, error)
}

type cache struct {
	maxSize int64
	ttl     time.Duration
	tagTTL  time.Duration

	cache *ristretto.Cache[string, any]
	group singleflight.Group
}

type Option = func(*cache)

func New(options ...Option) (Client, error) {
	c := &cache{}
	for _, opt := range options {
		opt(c)
	}
	if c.maxSize <= 0 {
		c.maxSize = DefaultMaxSize
	}
	rcache, err := ristretto.NewCache(&ristretto.Config[string, any]{
		MaxCost: c.maxSize,
		// ristretto recommends 10 frequency counters per entry the cache can hold
		NumCounters: 10 * max(c.maxSize/expectedEntryCost, 100),
		BufferItems: 64, // the value ristretto recommends
	})
	if err != nil {
		return nil, err
	}
	c.cache = rcache
	return c, nil
}

// WithMaxSize sets the budget in bytes; 0 selects DefaultMaxSize.
func WithMaxSize(s int64) Option { return func(c *cache) { c.maxSize = s } }

// WithTTLDuration sets the lifetime of digest entries; 0 disables digest
// caching, so digest references are always fetched.
func WithTTLDuration(t time.Duration) Option { return func(c *cache) { c.ttl = t } }

// WithTagTTLDuration sets how long a tag keeps resolving to the same digest;
// 0 disables tag caching, so tag references are always fetched.
func WithTagTTLDuration(t time.Duration) Option { return func(c *cache) { c.tagTTL = t } }

func (c *cache) Get(ctx context.Context, image string, fetch func(context.Context) (map[string]any, error)) (map[string]any, error) {
	ref, err := name.ParseReference(image)
	if err != nil {
		return fetch(ctx)
	}
	ttl := c.tagTTL
	if _, ok := ref.(name.Digest); ok {
		ttl = c.ttl
	}
	if ttl <= 0 {
		return fetch(ctx)
	}
	if v, ok := c.cache.Get(image); ok {
		if data, ok := v.(map[string]any); ok {
			record(ctx, metrics.ImageDataCacheHit)
			return data, nil
		}
	}
	var led bool // set only in the caller whose fetch ran; res.Shared is also true for it
	ch := c.group.DoChan(image, func() (_ any, err error) {
		led = true
		// singleflight re-raises a panic on a new goroutine, where nothing can recover it
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("fetching image data panicked: %v", r)
			}
		}()
		// the shared fetch must not fail its waiters when the first caller's deadline expires
		fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), fetchTimeout)
		defer cancel()
		data, err := fetch(fctx)
		if err != nil {
			return nil, err
		}
		c.store(image, data, ttl)
		return data, nil
	})
	select {
	case <-ctx.Done():
		record(ctx, metrics.ImageDataCacheError)
		return nil, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			record(ctx, metrics.ImageDataCacheError)
			return nil, res.Err
		}
		// a caller that waited on another caller's fetch made no registry call
		if led {
			record(ctx, metrics.ImageDataCacheMiss)
		} else {
			record(ctx, metrics.ImageDataCacheHit)
		}
		return res.Val.(map[string]any), nil
	}
}

func (c *cache) store(image string, data map[string]any, ttl time.Duration) {
	// shortcut: the entry cost is its JSON size, measured by encoding it on every
	// store; misses are rare, so upgrade only if profiling shows it.
	encoded, err := json.Marshal(data)
	if err != nil {
		return
	}
	c.cache.SetWithTTL(image, data, int64(len(encoded)), ttl)
	c.cache.Wait() // so the next request sees the entry
}

func record(ctx context.Context, result metrics.ImageDataCacheResult) {
	if m := metrics.GetImageDataCacheMetrics(); m != nil {
		m.RecordRequest(ctx, result)
	}
}
