package auth

import (
	"context"
	"strings"
	"sync"
)

// ResultCache memoizes authorization decisions for the duration of a single
// policy validation pass. It is created once per Validate() call and discarded
// when that call returns, so it never serves stale authorization decisions
// across policy updates or RBAC changes.
//
// Entries are keyed at the granularity of a single (subject, verb, resource)
// tuple — the same granularity at which Auth.CanI issues SubjectAccessReview
// requests (one per verb) — so an overlapping verb shared by different CanI
// calls (e.g. "get" in both mutate's {get,update} and generate's {get,create})
// is checked only once.
type ResultCache struct {
	mu      sync.Mutex
	results map[string]bool
}

// NewResultCache returns an empty ResultCache ready for use.
func NewResultCache() *ResultCache {
	return &ResultCache{
		results: make(map[string]bool),
	}
}

func (c *ResultCache) get(key string) (bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	allowed, ok := c.results[key]
	return allowed, ok
}

func (c *ResultCache) set(key string, allowed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results[key] = allowed
}

// cachedAuth wraps an AuthChecks and memoizes authorization decisions in a
// shared ResultCache. It evaluates each verb individually (mirroring Auth,
// which issues one SubjectAccessReview per verb) so overlapping verbs are not
// re-requested, then rebuilds the aggregate (allowed, message) result with the
// same buildMessage helper the underlying Auth uses — keeping the returned
// message byte-identical to the uncached path.
type cachedAuth struct {
	inner AuthChecks
	cache *ResultCache
}

// NewCachedAuth wraps inner so that per-verb authorization decisions within the
// same validation pass are answered from cache. It returns inner unchanged when
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
	user := c.inner.User()
	var failedVerbs []string
	for _, verb := range verbs {
		key := strings.Join([]string{user, verb, gvk, namespace, name, subresource}, "|")
		allowed, found := c.cache.get(key)
		if !found {
			// Resolve a single verb so the underlying Auth issues exactly one
			// SubjectAccessReview, which we can memoize per (subject, verb, resource).
			ok, _, err := c.inner.CanI(ctx, []string{verb}, gvk, namespace, name, subresource)
			if err != nil {
				// Never cache errors; the next call should retry.
				return false, "", err
			}
			allowed = ok
			c.cache.set(key, allowed)
		}
		if !allowed {
			failedVerbs = append(failedVerbs, verb)
		}
	}

	if len(failedVerbs) > 0 {
		return false, buildMessage(gvk, subresource, failedVerbs, user, namespace), nil
	}
	return true, "", nil
}
