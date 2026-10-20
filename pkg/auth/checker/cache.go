package checker

import (
	"context"
	"sync"

	authorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

// cacheKey identifies a single authorization decision at the granularity of one
// (subject, resolved-resource, verb) tuple — the granularity at which an
// AuthChecker issues a SubjectAccessReview/SelfSubjectAccessReview.
//
// The resource coordinates (group/version/resource) are the GVR the kind was
// resolved to before the checker ran, so different spellings of the same kind
// (e.g. "ConfigMap" and "v1/ConfigMap") collapse onto a single entry. subject
// keeps decisions for different identities from colliding in a shared cache, and
// self marks SelfSubjectAccessReview decisions so they occupy a key domain
// disjoint from every subject — a self decision can never be reused for a
// SubjectAccessReview even if a subject is literally named like the self value.
type cacheKey struct {
	self        bool
	subject     string
	group       string
	version     string
	resource    string
	subresource string
	namespace   string
	name        string
	verb        string
}

// ResultCache memoizes authorization decisions so a decorated AuthChecker is
// asked the same (identity, resource, verb) question only once.
//
// Contract — the cache is correct ONLY as a short-lived, single-pass cache:
//
//   - Lifetime is one validation pass. Create it once (e.g. per
//     pkg/validation/policy Validate() call) and discard it when that call
//     returns. It has no expiry or invalidation, so a longer-lived instance
//     would serve stale allow/deny decisions after an RBAC change — a
//     correctness/security bug. Never promote it to a controller- or
//     process-scoped cache.
//   - Keying is per (self, subject, resolved-GVR, subresource, namespace, name,
//     verb). subject is the SubjectAccessReview user; self puts
//     SelfSubjectAccessReview decisions in a disjoint domain.
//   - Errors are never cached (the next call retries); decisions are.
//   - It is not single-flight: concurrent callers that miss the same key each
//     issue their own review before the first result is stored. The mutex only
//     guards the map.
type ResultCache struct {
	mu      sync.Mutex
	results map[cacheKey]*AuthResult
}

// NewResultCache returns an empty ResultCache ready for use.
func NewResultCache() *ResultCache {
	return &ResultCache{
		results: make(map[cacheKey]*AuthResult),
	}
}

// get returns the cached decision for key and whether an entry exists.
func (c *ResultCache) get(key cacheKey) (*AuthResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	result, ok := c.results[key]
	return result, ok
}

// set records the authorization decision for key.
func (c *ResultCache) set(key cacheKey, result *AuthResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results[key] = result
}

// cachedChecker wraps an AuthChecker and answers repeated questions from a
// ResultCache. See ResultCache for the contract.
type cachedChecker struct {
	inner   AuthChecker
	cache   *ResultCache
	subject string
	self    bool
}

// newCachedChecker wraps inner with cache. The identity (subject/self) is
// supplied by the exported constructors, which also build inner, so the review
// identity and the cache-key identity come from a single source.
func newCachedChecker(inner AuthChecker, subject string, self bool, cache *ResultCache) *cachedChecker {
	return &cachedChecker{inner: inner, cache: cache, subject: subject, self: self}
}

// NewCachedSubjectChecker builds a SubjectAccessReview-based checker for user
// and memoizes its decisions in cache. Building the checker here keeps the
// review identity and the cache-key identity in one place — they cannot drift.
// Returns a plain (uncached) checker when cache is nil.
func NewCachedSubjectChecker(client authorizationv1client.SubjectAccessReviewInterface, user string, cache *ResultCache) AuthChecker {
	inner := NewSubjectChecker(client, user, nil)
	if cache == nil {
		return inner
	}
	return newCachedChecker(inner, user, false, cache)
}

// NewCachedSelfChecker builds a SelfSubjectAccessReview-based checker and
// memoizes its decisions in cache, in a key domain disjoint from every subject
// (a self decision is never reused for a subject's SubjectAccessReview, and vice
// versa). Returns a plain (uncached) checker when cache is nil.
func NewCachedSelfChecker(client authorizationv1client.SelfSubjectAccessReviewInterface, cache *ResultCache) AuthChecker {
	inner := NewSelfChecker(client)
	if cache == nil {
		return inner
	}
	return newCachedChecker(inner, "", true, cache)
}

// Check returns the cached decision for the (subject, resource, verb) tuple when
// present, resolving it through the inner checker (one access review) on a miss.
// Errors are never cached so the next call retries.
func (c *cachedChecker) Check(ctx context.Context, group, version, resource, subresource, namespace, name, verb string) (*AuthResult, error) {
	key := cacheKey{
		self:        c.self,
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
