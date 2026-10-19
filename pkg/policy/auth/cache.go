package auth

import (
	"context"
	"sync"

	"github.com/kyverno/kyverno/pkg/auth/checker"
)

// cacheKey identifies a single authorization decision at the granularity of one
// (subject, resolved-resource, verb) tuple — the granularity at which
// checker.AuthChecker issues a SubjectAccessReview/SelfSubjectAccessReview.
//
// The resource coordinates (group/version/resource) are the GVR the kind was
// resolved to before the checker ran, so different spellings of the same kind
// (e.g. "ConfigMap" and "v1/ConfigMap") collapse onto a single entry. subject
// keeps decisions for different identities from colliding in a shared cache.
type cacheKey struct {
	subject     string
	group       string
	version     string
	resource    string
	subresource string
	namespace   string
	name        string
	verb        string
}

// ResultCache memoizes authorization decisions for the duration of a single
// policy validation pass. It is created once per Validate() call and discarded
// when that call returns, so it never serves stale authorization decisions
// across policy updates or RBAC changes.
type ResultCache struct {
	mu      sync.Mutex
	results map[cacheKey]*checker.AuthResult
}

// NewResultCache returns an empty ResultCache ready for use.
func NewResultCache() *ResultCache {
	return &ResultCache{
		results: make(map[cacheKey]*checker.AuthResult),
	}
}

// get returns the cached decision for key and whether an entry exists.
func (c *ResultCache) get(key cacheKey) (*checker.AuthResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	result, ok := c.results[key]
	return result, ok
}

// set records the authorization decision for key.
func (c *ResultCache) set(key cacheKey, result *checker.AuthResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results[key] = result
}

// cachedChecker wraps a checker.AuthChecker and memoizes its decisions in a
// shared ResultCache. Caching at this layer — below pkg/policy/auth.Auth.CanI
// and the pkg/admissionpolicy self-checks — keys on the resolved GVR and returns
// the inner checker's AuthResult verbatim, so every caller (CanI's aggregate
// message as well as the RunAccessCheck callers that surface Reason) sees the
// same result it would without the cache. This mirrors the upstream Kubernetes
// NewCachingAuthorizer pattern.
type cachedChecker struct {
	inner   checker.AuthChecker
	cache   *ResultCache
	subject string
}

// NewCachedChecker wraps inner so that authorization decisions within the same
// validation pass are answered from cache. subject is the identity inner checks
// as — the SubjectAccessReview user, or a stable sentinel for a self-checker —
// and is folded into the cache key so one cache can be shared across subjects.
// It returns inner unchanged when either inner or cache is nil (callers rely on
// a nil inner to skip checks).
func NewCachedChecker(inner checker.AuthChecker, subject string, cache *ResultCache) checker.AuthChecker {
	if inner == nil || cache == nil {
		return inner
	}
	return &cachedChecker{inner: inner, cache: cache, subject: subject}
}

// Check returns the cached decision for the (subject, resource, verb) tuple when
// present, resolving it through the inner checker (one access review) on a miss.
// Errors are never cached so the next call retries.
func (c *cachedChecker) Check(ctx context.Context, group, version, resource, subresource, namespace, name, verb string) (*checker.AuthResult, error) {
	key := cacheKey{
		subject:     c.subject,
		group:       group,
		version:     version,
		resource:    resource,
		subresource: subresource,
		namespace:   namespace,
		name:        name,
		verb:        verb,
	}
	if result, ok := c.cache.get(key); ok {
		return result, nil
	}
	result, err := c.inner.Check(ctx, group, version, resource, subresource, namespace, name, verb)
	if err != nil {
		return nil, err
	}
	c.cache.set(key, result)
	return result, nil
}
