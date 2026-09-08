package cache

import (
	"context"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultTTL     = 1 * time.Hour
	defaultMaxSize = 1000
)

type cache struct {
	logger         logr.Logger
	isCacheEnabled bool
	maxSize        int64
	ttl            time.Duration
	cache          *ristretto.Cache[string, any]
}

type Option = func(*cache) error

func New(options ...Option) (Client, error) {
	cache := &cache{}
	for _, opt := range options {
		if err := opt(cache); err != nil {
			return nil, err
		}
	}
	config := ristretto.Config[string, any]{
		MaxCost:     cache.maxSize,
		NumCounters: 10 * cache.maxSize,
		BufferItems: 64,
	}
	rcache, err := ristretto.NewCache(&config)
	if err != nil {
		return nil, err
	}
	cache.cache = rcache
	return cache, nil
}

func DisabledImageVerifyCache() Client {
	return &cache{
		logger:         logr.Discard(),
		isCacheEnabled: false,
		maxSize:        0,
		ttl:            0,
	}
}

func WithLogger(l logr.Logger) Option {
	return func(c *cache) error {
		c.logger = l
		return nil
	}
}

func WithCacheEnableFlag(b bool) Option {
	return func(c *cache) error {
		c.isCacheEnabled = b
		return nil
	}
}

func WithMaxSize(s int64) Option {
	return func(c *cache) error {
		if s == 0 {
			s = defaultMaxSize
		}
		c.maxSize = s
		return nil
	}
}

func WithTTLDuration(t time.Duration) Option {
	return func(c *cache) error {
		if t == 0 {
			t = defaultTTL
		}
		c.ttl = t
		return nil
	}
}

func generateKey(policy metav1.Object, ruleName string, imageRef string) string {
	return string(policy.GetUID()) + ";" + policy.GetResourceVersion() + ";" + ruleName + ";" + imageRef
}

func (c *cache) Set(ctx context.Context, policy metav1.Object, ruleName string, imageRef string, useCache bool) (bool, error) {
	return c.SetWithPayload(ctx, policy, ruleName, imageRef, useCache, nil)
}

func (c *cache) Get(ctx context.Context, policy metav1.Object, ruleName string, imageRef string, useCache bool) (bool, error) {
	found, _, err := c.GetWithPayload(ctx, policy, ruleName, imageRef, useCache)
	return found, err
}

func (c *cache) SetWithPayload(ctx context.Context, policy metav1.Object, ruleName string, imageRef string, useCache bool, payloads map[string][]byte) (bool, error) {
	if !c.isCacheEnabled {
		// If cache is globally disabled just return
		return false, nil
	} else if !useCache {
		// Else If enabled globally then return if locally disabled
		return false, nil
	}
	key := generateKey(policy, ruleName, imageRef)

	stored := c.cache.SetWithTTL(key, clonePayloads(payloads), payloadCost(payloads), c.ttl)
	c.cache.Wait()
	if stored {
		return true, nil
	}
	return false, nil
}

func (c *cache) GetWithPayload(ctx context.Context, policy metav1.Object, ruleName string, imageRef string, useCache bool) (bool, map[string][]byte, error) {
	if !c.isCacheEnabled {
		// If cache is globally disabled just return
		return false, nil, nil
	} else if !useCache {
		// Else If enabled globally then return if locally disabled
		return false, nil, nil
	}
	key := generateKey(policy, ruleName, imageRef)
	val, found := c.cache.Get(key)
	if !found {
		return false, nil, nil
	}
	payloads, _ := val.(map[string][]byte)
	return true, clonePayloads(payloads), nil
}

// payloadCost estimates the memory footprint of a cache entry so ristretto's
// MaxCost (--imageVerifyCacheMaxSize) bounds actual memory rather than just
// entry count: a presence-only entry (nil payloads) still costs 1, while an
// entry carrying attestation payload bytes costs roughly its real size.
func payloadCost(payloads map[string][]byte) int64 {
	cost := int64(1)
	for _, v := range payloads {
		cost += int64(len(v))
	}
	return cost
}

func clonePayloads(in map[string][]byte) map[string][]byte {
	if in == nil {
		return nil
	}
	out := make(map[string][]byte, len(in))
	for k, v := range in {
		if v == nil {
			out[k] = nil
			continue
		}
		cp := make([]byte, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}
