package checker

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// countingChecker is a test AuthChecker that records every Check invocation and
// denies any verb listed in denied.
type countingChecker struct {
	denied map[string]bool
	err    error
	mu     sync.Mutex
	calls  int
}

// Check records the invocation, then denies any verb present in denied or
// returns the configured error.
func (c *countingChecker) Check(_ context.Context, group, version, resource, subresource, namespace, name, verb string) (*AuthResult, error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	if c.err != nil {
		return nil, c.err
	}
	return &AuthResult{Allowed: !c.denied[verb], Reason: "reason:" + verb}, nil
}

func (c *countingChecker) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

// TestNewCachedChecker_NilPassthrough verifies the wrapper is a no-op when either
// the inner checker or the cache is nil.
func TestNewCachedChecker_NilPassthrough(t *testing.T) {
	inner := &countingChecker{}

	assert.Nil(t, NewCachedChecker(nil, "u", NewResultCache()))
	assert.Same(t, inner, NewCachedChecker(inner, "u", nil))
}

// TestCachedChecker_MemoizesIdenticalCalls verifies repeated identical Check
// calls resolve through the inner checker only once.
func TestCachedChecker_MemoizesIdenticalCalls(t *testing.T) {
	inner := &countingChecker{}
	c := NewCachedChecker(inner, "reports-sa", NewResultCache())
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		result, err := c.Check(ctx, "", "v1", "configmaps", "", "", "", "get")
		assert.NoError(t, err)
		assert.True(t, result.Allowed)
	}

	assert.Equal(t, 1, inner.callCount())
}

// TestCachedChecker_DistinctKeysMiss verifies that varying any component of the
// cache key produces a fresh lookup against the inner checker.
func TestCachedChecker_DistinctKeysMiss(t *testing.T) {
	inner := &countingChecker{}
	c := NewCachedChecker(inner, "reports-sa", NewResultCache())
	ctx := context.Background()

	_, _ = c.Check(ctx, "", "v1", "configmaps", "", "", "", "get")
	_, _ = c.Check(ctx, "apps", "v1", "deployments", "", "", "", "get")  // resource
	_, _ = c.Check(ctx, "", "v1", "configmaps", "status", "", "", "get") // subresource
	_, _ = c.Check(ctx, "", "v1", "configmaps", "", "ns", "", "get")     // namespace
	_, _ = c.Check(ctx, "", "v1", "configmaps", "", "", "name", "get")   // name
	_, _ = c.Check(ctx, "", "v1", "configmaps", "", "", "", "list")      // verb

	assert.Equal(t, 6, inner.callCount())
}

// TestCachedChecker_SharedCacheKeyedBySubject verifies a cache shared between
// checkers for different subjects does not let one subject's decision satisfy
// another's.
func TestCachedChecker_SharedCacheKeyedBySubject(t *testing.T) {
	cache := NewResultCache()
	reports := &countingChecker{}
	background := &countingChecker{}
	reportsChecker := NewCachedChecker(reports, "reports-sa", cache)
	backgroundChecker := NewCachedChecker(background, "background-sa", cache)
	ctx := context.Background()

	// Same tuple but different subjects must not collide in the shared cache.
	_, _ = reportsChecker.Check(ctx, "", "v1", "configmaps", "", "", "", "get")
	_, _ = reportsChecker.Check(ctx, "", "v1", "configmaps", "", "", "", "get")
	_, _ = backgroundChecker.Check(ctx, "", "v1", "configmaps", "", "", "", "get")

	assert.Equal(t, 1, reports.callCount())
	assert.Equal(t, 1, background.callCount())
}

// TestNewCachedSelfChecker_NilPassthrough verifies the self wrapper is a no-op
// when either the inner checker or the cache is nil.
func TestNewCachedSelfChecker_NilPassthrough(t *testing.T) {
	inner := &countingChecker{}
	assert.Nil(t, NewCachedSelfChecker(nil, NewResultCache()))
	assert.Same(t, inner, NewCachedSelfChecker(inner, nil))
}

// TestCachedSelfChecker_DisjointFromSubject verifies a SelfSubjectAccessReview
// decision is never reused for a subject's SubjectAccessReview on the same
// resource/verb — even when a subject is literally named like the self value.
func TestCachedSelfChecker_DisjointFromSubject(t *testing.T) {
	cache := NewResultCache()
	self := &countingChecker{}
	subject := &countingChecker{}
	selfChecker := NewCachedSelfChecker(self, cache)
	subjectChecker := NewCachedChecker(subject, "self", cache) // subject literally "self"
	ctx := context.Background()

	_, _ = selfChecker.Check(ctx, "admissionregistration.k8s.io", "v1", "validatingadmissionpolicies", "", "", "", "create")
	_, _ = subjectChecker.Check(ctx, "admissionregistration.k8s.io", "v1", "validatingadmissionpolicies", "", "", "", "create")

	// Each inner checker is consulted once; the self decision did not satisfy the
	// subject's identical tuple, nor vice versa.
	assert.Equal(t, 1, self.callCount())
	assert.Equal(t, 1, subject.callCount())
}

// TestCachedChecker_DoesNotCacheErrors verifies an errored check is not memoized
// and is retried on the next call.
func TestCachedChecker_DoesNotCacheErrors(t *testing.T) {
	inner := &countingChecker{err: errors.New("boom")}
	c := NewCachedChecker(inner, "reports-sa", NewResultCache())
	ctx := context.Background()

	_, err1 := c.Check(ctx, "", "v1", "configmaps", "", "", "", "get")
	_, err2 := c.Check(ctx, "", "v1", "configmaps", "", "", "", "get")

	assert.Error(t, err1)
	assert.Error(t, err2)
	assert.Equal(t, 2, inner.callCount())
}

// TestCachedChecker_ReturnsResultVerbatim verifies the inner checker's AuthResult
// (including Reason) is returned unchanged on both the miss and the cache hit —
// nothing is dropped or rebuilt at this layer.
func TestCachedChecker_ReturnsResultVerbatim(t *testing.T) {
	inner := &countingChecker{denied: map[string]bool{"delete": true}}
	c := NewCachedChecker(inner, "reports-sa", NewResultCache())
	ctx := context.Background()

	miss, err := c.Check(ctx, "", "v1", "configmaps", "", "", "", "delete")
	assert.NoError(t, err)
	assert.False(t, miss.Allowed)
	assert.Equal(t, "reason:delete", miss.Reason)

	hit, err := c.Check(ctx, "", "v1", "configmaps", "", "", "", "delete")
	assert.NoError(t, err)
	assert.Equal(t, miss, hit)
	assert.Equal(t, 1, inner.callCount())
}
