package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// countingAuth is a test AuthChecks that records every CanI invocation and the
// verbs it was asked about, and denies any verb listed in denied.
type countingAuth struct {
	user      string
	denied    map[string]bool
	err       error
	calls     int
	verbCalls []string
}

func (c *countingAuth) User() string { return c.user }

func (c *countingAuth) CanI(_ context.Context, verbs []string, gvk, namespace, name, subresource string) (bool, string, error) {
	c.calls++
	c.verbCalls = append(c.verbCalls, verbs...)
	if c.err != nil {
		return false, "", c.err
	}
	var failed []string
	for _, v := range verbs {
		if c.denied[v] {
			failed = append(failed, v)
		}
	}
	if len(failed) > 0 {
		return false, buildMessage(gvk, subresource, failed, c.user, namespace), nil
	}
	return true, "", nil
}

func countVerb(verbCalls []string, verb string) int {
	n := 0
	for _, v := range verbCalls {
		if v == verb {
			n++
		}
	}
	return n
}

func TestNewCachedAuth_NilPassthrough(t *testing.T) {
	inner := &countingAuth{user: "u"}

	assert.Nil(t, NewCachedAuth(nil, NewResultCache()))
	assert.Same(t, inner, NewCachedAuth(inner, nil))
}

func TestCachedAuth_MemoizesIdenticalCalls(t *testing.T) {
	inner := &countingAuth{user: "reports-sa"}
	checker := NewCachedAuth(inner, NewResultCache())
	ctx := context.Background()
	verbs := []string{"get", "list", "watch"}

	for i := 0; i < 5; i++ {
		ok, msg, err := checker.CanI(ctx, verbs, "Pod", "", "", "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Empty(t, msg)
	}

	// Each of the three verbs is resolved exactly once across all five calls.
	assert.Equal(t, 3, inner.calls)
	assert.Equal(t, "reports-sa", checker.User())
}

// TestCachedAuth_DeduplicatesOverlappingVerbs is the regression test for the
// per-verb keying fix: a verb shared by two different CanI calls ("get" in both
// {get,update} and {get,create}) must only be requested once.
func TestCachedAuth_DeduplicatesOverlappingVerbs(t *testing.T) {
	inner := &countingAuth{user: "background-sa"}
	checker := NewCachedAuth(inner, NewResultCache())
	ctx := context.Background()

	_, _, _ = checker.CanI(ctx, []string{"get", "update"}, "Pod", "", "", "")
	_, _, _ = checker.CanI(ctx, []string{"get", "create"}, "Pod", "", "", "")

	// get + update + create = three distinct checks; "get" is not repeated.
	assert.Equal(t, 3, inner.calls)
	assert.Equal(t, 1, countVerb(inner.verbCalls, "get"))
	assert.Equal(t, 1, countVerb(inner.verbCalls, "update"))
	assert.Equal(t, 1, countVerb(inner.verbCalls, "create"))
}

func TestCachedAuth_DistinctKeysMiss(t *testing.T) {
	inner := &countingAuth{user: "reports-sa"}
	checker := NewCachedAuth(inner, NewResultCache())
	ctx := context.Background()
	v := []string{"get"}

	// Vary each component of the key in turn; every variation is a fresh miss.
	_, _, _ = checker.CanI(ctx, v, "Pod", "", "", "")
	_, _, _ = checker.CanI(ctx, v, "Deployment", "", "", "") // gvk
	_, _, _ = checker.CanI(ctx, v, "Pod", "ns", "", "")      // namespace
	_, _, _ = checker.CanI(ctx, v, "Pod", "", "name", "")    // name
	_, _, _ = checker.CanI(ctx, v, "Pod", "", "", "status")  // subresource

	assert.Equal(t, 5, inner.calls)
}

func TestCachedAuth_SharedCacheKeyedByUser(t *testing.T) {
	cache := NewResultCache()
	reports := &countingAuth{user: "reports-sa"}
	background := &countingAuth{user: "background-sa"}
	reportsChecker := NewCachedAuth(reports, cache)
	backgroundChecker := NewCachedAuth(background, cache)
	ctx := context.Background()
	verbs := []string{"get", "list", "watch"}

	// Same tuple but different subjects must not collide in the shared cache.
	_, _, _ = reportsChecker.CanI(ctx, verbs, "Pod", "", "", "")
	_, _, _ = reportsChecker.CanI(ctx, verbs, "Pod", "", "", "")
	_, _, _ = backgroundChecker.CanI(ctx, verbs, "Pod", "", "", "")

	assert.Equal(t, 3, reports.calls)
	assert.Equal(t, 3, background.calls)
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
	// First verb errors and short-circuits each call; nothing is cached.
	assert.Equal(t, 2, inner.calls)
}

// TestCachedAuth_DenialMessageMatchesUncached asserts the aggregated denial
// message is byte-identical to the uncached batch path, and that denials are
// cached (no repeat requests).
func TestCachedAuth_DenialMessageMatchesUncached(t *testing.T) {
	ctx := context.Background()
	verbs := []string{"get", "list", "watch"}
	denied := map[string]bool{"list": true, "watch": true}

	// Uncached reference: a single batched CanI call.
	reference := &countingAuth{user: "reports-sa", denied: denied}
	wantOK, wantMsg, _ := reference.CanI(ctx, verbs, "Pod", "", "", "")

	inner := &countingAuth{user: "reports-sa", denied: denied}
	checker := NewCachedAuth(inner, NewResultCache())

	gotOK, gotMsg, err := checker.CanI(ctx, verbs, "Pod", "", "", "")
	assert.NoError(t, err)
	assert.Equal(t, wantOK, gotOK)
	assert.Equal(t, wantMsg, gotMsg)

	// Second call is fully served from cache and returns the same message.
	gotOK2, gotMsg2, _ := checker.CanI(ctx, verbs, "Pod", "", "", "")
	assert.False(t, gotOK2)
	assert.Equal(t, wantMsg, gotMsg2)
	assert.Equal(t, 3, inner.calls)
}
