package auth

import (
	"context"
	"strings"
	"sync"
)

// ResultCache memoizes CanI results for the duration of a single policy
// validation pass. It is created once per Validate() call and discarded when
// that call returns, so it never serves stale authorization decisions across
// policy updates or RBAC changes.
type ResultCache struct {
	mu      sync.Mutex
	results map[string]canIResult
}

type canIResult struct {
	ok  bool
	msg string
}

// NewResultCache returns an empty ResultCache ready for use.
func NewResultCache() *ResultCache {
	return &ResultCache{
		results: make(map[string]canIResult),
	}
}

func (c *ResultCache) get(key string) (canIResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.results[key]
	return r, ok
}

func (c *ResultCache) set(key string, r canIResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results[key] = r
}

// cachedAuth wraps an AuthChecks and memoizes CanI results in a shared
// ResultCache. The cache key is prefixed with the subject so a single cache can
// safely serve checkers for different service accounts without collisions.
type cachedAuth struct {
	inner AuthChecks
	cache *ResultCache
}

// NewCachedAuth wraps inner so that identical CanI calls within the same
// validation pass are answered from cache. It returns inner unchanged when
// either cache or inner is nil (callers rely on a nil inner to skip checks).
func NewCachedAuth(inner AuthChecks, cache *ResultCache) AuthChecks {
	if inner == nil || cache == nil {
		return inner
	}
	return &cachedAuth{inner: inner, cache: cache}
}

func (c *cachedAuth) User() string {
	return c.inner.User()
}

func (c *cachedAuth) CanI(ctx context.Context, verbs []string, gvk, namespace, name, subresource string) (bool, string, error) {
	key := strings.Join([]string{c.inner.User(), strings.Join(verbs, ","), gvk, namespace, name, subresource}, "|")
	if r, ok := c.cache.get(key); ok {
		return r.ok, r.msg, nil
	}

	ok, msg, err := c.inner.CanI(ctx, verbs, gvk, namespace, name, subresource)
	if err != nil {
		// Never cache errors; the next call should retry.
		return ok, msg, err
	}

	c.cache.set(key, canIResult{ok: ok, msg: msg})
	return ok, msg, nil
}
