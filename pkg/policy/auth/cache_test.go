package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// countingAuth is a test AuthChecks that records how many times CanI is called.
type countingAuth struct {
	user  string
	ok    bool
	msg   string
	err   error
	calls int
}

func (c *countingAuth) User() string { return c.user }

func (c *countingAuth) CanI(_ context.Context, _ []string, _, _, _, _ string) (bool, string, error) {
	c.calls++
	return c.ok, c.msg, c.err
}

func TestNewCachedAuth_NilPassthrough(t *testing.T) {
	inner := &countingAuth{user: "u", ok: true}

	assert.Nil(t, NewCachedAuth(nil, NewResultCache()))
	assert.Same(t, inner, NewCachedAuth(inner, nil))
}

func TestCachedAuth_MemoizesIdenticalCalls(t *testing.T) {
	inner := &countingAuth{user: "reports-sa", ok: true}
	checker := NewCachedAuth(inner, NewResultCache())
	ctx := context.Background()
	verbs := []string{"get", "list", "watch"}

	for i := 0; i < 5; i++ {
		ok, msg, err := checker.CanI(ctx, verbs, "Pod", "", "", "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Empty(t, msg)
	}

	assert.Equal(t, 1, inner.calls, "identical CanI calls should hit the inner checker exactly once")
	assert.Equal(t, "reports-sa", checker.User())
}

func TestCachedAuth_DistinctKeysMiss(t *testing.T) {
	inner := &countingAuth{user: "reports-sa", ok: true}
	checker := NewCachedAuth(inner, NewResultCache())
	ctx := context.Background()
	verbs := []string{"get", "list", "watch"}

	// Vary each component of the key in turn; every variation is a fresh miss.
	_, _, _ = checker.CanI(ctx, verbs, "Pod", "", "", "")
	_, _, _ = checker.CanI(ctx, verbs, "Deployment", "", "", "")    // gvk
	_, _, _ = checker.CanI(ctx, []string{"get"}, "Pod", "", "", "") // verbs
	_, _, _ = checker.CanI(ctx, verbs, "Pod", "ns", "", "")         // namespace
	_, _, _ = checker.CanI(ctx, verbs, "Pod", "", "name", "")       // name
	_, _, _ = checker.CanI(ctx, verbs, "Pod", "", "", "status")     // subresource

	assert.Equal(t, 6, inner.calls)
}

func TestCachedAuth_SharedCacheKeyedByUser(t *testing.T) {
	cache := NewResultCache()
	reports := &countingAuth{user: "reports-sa", ok: true}
	background := &countingAuth{user: "background-sa", ok: true}
	reportsChecker := NewCachedAuth(reports, cache)
	backgroundChecker := NewCachedAuth(background, cache)
	ctx := context.Background()
	verbs := []string{"get", "list", "watch"}

	// Same tuple but different subjects must not collide in the shared cache.
	_, _, _ = reportsChecker.CanI(ctx, verbs, "Pod", "", "", "")
	_, _, _ = reportsChecker.CanI(ctx, verbs, "Pod", "", "", "")
	_, _, _ = backgroundChecker.CanI(ctx, verbs, "Pod", "", "", "")

	assert.Equal(t, 1, reports.calls)
	assert.Equal(t, 1, background.calls)
}

func TestCachedAuth_DoesNotCacheErrors(t *testing.T) {
	inner := &countingAuth{user: "reports-sa", err: errors.New("boom")}
	checker := NewCachedAuth(inner, NewResultCache())
	ctx := context.Background()
	verbs := []string{"get", "list", "watch"}

	_, _, err1 := checker.CanI(ctx, verbs, "Pod", "", "", "")
	_, _, err2 := checker.CanI(ctx, verbs, "Pod", "", "", "")

	assert.Error(t, err1)
	assert.Error(t, err2)
	assert.Equal(t, 2, inner.calls, "errors must not be cached; the next call retries")
}

func TestCachedAuth_CachesDenials(t *testing.T) {
	inner := &countingAuth{user: "reports-sa", ok: false, msg: "denied"}
	checker := NewCachedAuth(inner, NewResultCache())
	ctx := context.Background()
	verbs := []string{"get", "list", "watch"}

	ok1, msg1, _ := checker.CanI(ctx, verbs, "Pod", "", "", "")
	ok2, msg2, _ := checker.CanI(ctx, verbs, "Pod", "", "", "")

	assert.False(t, ok1)
	assert.False(t, ok2)
	assert.Equal(t, "denied", msg1)
	assert.Equal(t, "denied", msg2)
	assert.Equal(t, 1, inner.calls)
}
