package auth

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/auth/checker"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

// countingChecker is a test checker.AuthChecker that records every Check
// invocation and denies any verb listed in denied.
type countingChecker struct {
	denied map[string]bool
	err    error
	mu     sync.Mutex
	calls  int
	verbs  []string
}

// Check records the invocation and its verb, then denies any verb present in
// denied or returns the configured error.
func (c *countingChecker) Check(_ context.Context, group, version, resource, subresource, namespace, name, verb string) (*checker.AuthResult, error) {
	c.mu.Lock()
	c.calls++
	c.verbs = append(c.verbs, verb)
	c.mu.Unlock()
	if c.err != nil {
		return nil, c.err
	}
	return &checker.AuthResult{Allowed: !c.denied[verb], Reason: "reason:" + verb}, nil
}

func (c *countingChecker) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

// countVerb returns how many times verb was checked.
func (c *countingChecker) countVerb(verb string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, v := range c.verbs {
		if v == verb {
			n++
		}
	}
	return n
}

// countingSAR wraps a fake kube clientset with a SubjectAccessReview reactor
// that records every request and allows all of them, so a test can assert how
// many SubjectAccessReviews were actually issued.
func countingSAR() (*kubefake.Clientset, func() int) {
	kube := kubefake.NewSimpleClientset()
	var mu sync.Mutex
	count := 0
	kube.PrependReactor("create", "subjectaccessreviews", func(action clienttesting.Action) (bool, runtime.Object, error) {
		mu.Lock()
		count++
		mu.Unlock()
		review := action.(clienttesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
		review.Status.Allowed = true
		return true, review, nil
	})
	return kube, func() int {
		mu.Lock()
		defer mu.Unlock()
		return count
	}
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

// TestAuth_CollapsesKindSpellings drives the full Auth.CanI -> NewCanI ->
// RunAccessCheck path so GVK->GVR resolution runs before the cache. "ConfigMap"
// and "v1/ConfigMap" resolve to the same GVR, so the second call is a cache hit
// and issues no additional SubjectAccessReview — the key benefit of caching at
// the checker layer instead of on the raw kind string.
func TestAuth_CollapsesKindSpellings(t *testing.T) {
	kube, requests := countingSAR()
	client := dclient.NewFakeClientWithDisco(nil, kube, dclient.NewFakeDiscoveryClient(nil))
	a := NewAuth(client, "reports-sa", logr.Discard(), NewResultCache())
	ctx := context.Background()

	ok1, _, err := a.CanI(ctx, []string{"get"}, "ConfigMap", "", "", "")
	assert.NoError(t, err)
	assert.True(t, ok1)

	ok2, _, err := a.CanI(ctx, []string{"get"}, "v1/ConfigMap", "", "", "")
	assert.NoError(t, err)
	assert.True(t, ok2)

	assert.Equal(t, 1, requests())
}

// TestAuth_SharedCacheDeduplicatesAcrossCheckers verifies the main motivation of
// the PR: a single cache shared between two Auth instances for the same subject
// (as the mutate/validate/generate factories build) issues each distinct
// (resource, verb) SubjectAccessReview only once.
func TestAuth_SharedCacheDeduplicatesAcrossCheckers(t *testing.T) {
	kube, requests := countingSAR()
	client := dclient.NewFakeClientWithDisco(nil, kube, dclient.NewFakeDiscoveryClient(nil))
	cache := NewResultCache()
	first := NewAuth(client, "reports-sa", logr.Discard(), cache)
	second := NewAuth(client, "reports-sa", logr.Discard(), cache)
	ctx := context.Background()

	_, _, _ = first.CanI(ctx, []string{"get", "list"}, "ConfigMap", "", "", "")
	_, _, _ = second.CanI(ctx, []string{"get", "list"}, "ConfigMap", "", "", "")

	// get + list = two distinct checks; the second Auth reuses both from cache.
	assert.Equal(t, 2, requests())
}
