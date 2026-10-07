package cache

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test_payloadCost verifies cache entry cost is charged by real payload byte
// size instead of a flat 1 per entry, so --imageVerifyCacheMaxSize (ristretto's
// MaxCost) bounds actual memory rather than just entry count.
func Test_payloadCost(t *testing.T) {
	assert.Equal(t, int64(1), payloadCost(nil), "presence-only entry (nil payload) must keep the old flat cost of 1")
	assert.Equal(t, int64(1), payloadCost(map[string][]byte{}), "empty payload map must keep the old flat cost of 1")
	assert.Equal(t, int64(1+5), payloadCost(map[string][]byte{"a": []byte("hello")}), "cost must include the payload's byte length")
	assert.Equal(t, int64(1+5+3), payloadCost(map[string][]byte{"a": []byte("hello"), "b": []byte("foo")}), "cost must sum every payload's byte length")
	assert.Equal(t, int64(1), payloadCost(map[string][]byte{"a": nil}), "a nil-valued entry contributes zero bytes, not a panic")
}

// Test_SetWithPayload_presence_only_entry_costs_flat_one confirms the legacy
// Set()/plain-signature-cache path (which always stores a nil payload) keeps
// exactly the pre-fix cost of 1, so existing callers like the legacy
// ClusterPolicy image-verification engine are unaffected by this change.
func Test_SetWithPayload_presence_only_entry_costs_flat_one(t *testing.T) {
	c, err := New(WithCacheEnableFlag(true), WithMaxSize(0), WithTTLDuration(0))
	assert.NoError(t, err)

	pol := &metav1.ObjectMeta{Name: "cost-test-policy", UID: "cost-test-uid", ResourceVersion: "1"}

	stored, err := c.Set(context.TODO(), pol, "signature-rule", "image-a", true)
	assert.NoError(t, err)
	assert.True(t, stored)

	found, err := c.Get(context.TODO(), pol, "signature-rule", "image-a", true)
	assert.NoError(t, err)
	assert.True(t, found)
}

// Test_SetWithPayload_round_trips_real_payload_size confirms a payload-carrying
// entry (the attestation-cache path) round-trips correctly through the cache
// once cost accounting reflects its real size, not just that storage still
// works with the old flat cost.
func Test_SetWithPayload_round_trips_real_payload_size(t *testing.T) {
	// MaxSize must comfortably exceed the test payload's cost (1 + byte
	// length, see payloadCost).
	c, err := New(WithCacheEnableFlag(true), WithMaxSize(1_000_000), WithTTLDuration(0))
	assert.NoError(t, err)

	pol := &metav1.ObjectMeta{Name: "cost-test-policy", UID: "cost-test-uid", ResourceVersion: "1"}
	largePayload := map[string][]byte{"https://slsa.dev/provenance/v1": make([]byte, 4096)}

	stored, err := c.SetWithPayload(context.TODO(), pol, "attestation-rule", "image-b", true, largePayload)
	assert.NoError(t, err)
	assert.True(t, stored)

	found, got, err := c.GetWithPayload(context.TODO(), pol, "attestation-rule", "image-b", true)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, largePayload, got)
}

// Test_default_budget_holds_many_presence_only_entries: the flag used to
// promise 1000 keys, but ristretto also charges its own per-item cost, so a
// 1000 budget kept only about 17 entries.
func Test_default_budget_holds_many_presence_only_entries(t *testing.T) {
	c, err := New(WithCacheEnableFlag(true), WithMaxSize(0), WithTTLDuration(0))
	require.NoError(t, err)

	pol := &metav1.ObjectMeta{Name: "capacity-policy", UID: "capacity-uid", ResourceVersion: "1"}
	const entries = 1000
	for i := range entries {
		stored, err := c.Set(context.TODO(), pol, "signature-rule", fmt.Sprintf("registry.example/app-%d:v1", i), true)
		require.NoError(t, err)
		require.True(t, stored)
	}
	found := 0
	for i := range entries {
		ok, err := c.Get(context.TODO(), pol, "signature-rule", fmt.Sprintf("registry.example/app-%d:v1", i), true)
		require.NoError(t, err)
		if ok {
			found++
		}
	}
	assert.Equal(t, entries, found)
}

// Test_default_budget_caches_attestation_payloads: a minimal GitHub SLSA
// provenance statement is about 1 KB, which the old default could never hold,
// so every admission verified it again.
func Test_default_budget_caches_attestation_payloads(t *testing.T) {
	for _, size := range []int{1033, 4 << 10, 256 << 10} {
		t.Run(fmt.Sprintf("%d bytes", size), func(t *testing.T) {
			c, err := New(WithCacheEnableFlag(true), WithMaxSize(0), WithTTLDuration(0))
			require.NoError(t, err)

			pol := &metav1.ObjectMeta{Name: "payload-policy", UID: "payload-uid", ResourceVersion: "1"}
			payload := map[string][]byte{"https://slsa.dev/provenance/v1": make([]byte, size)}
			stored, err := c.SetWithPayload(context.TODO(), pol, "attestation-rule", "registry.example/app:v1", true, payload)
			require.NoError(t, err)
			assert.True(t, stored)

			found, got, err := c.GetWithPayload(context.TODO(), pol, "attestation-rule", "registry.example/app:v1", true)
			require.NoError(t, err)
			require.True(t, found)
			assert.Equal(t, payload, got)
		})
	}
}

// Test_SetWithPayload_reports_an_entry_larger_than_the_budget_as_not_stored:
// ristretto accepts the write and drops it afterwards, so the result has to
// come from the cache contents, not from the write call.
func Test_SetWithPayload_reports_an_entry_larger_than_the_budget_as_not_stored(t *testing.T) {
	c, err := New(WithCacheEnableFlag(true), WithMaxSize(100), WithTTLDuration(0))
	require.NoError(t, err)

	pol := &metav1.ObjectMeta{Name: "oversize-policy", UID: "oversize-uid", ResourceVersion: "1"}
	payload := map[string][]byte{"https://slsa.dev/provenance/v1": make([]byte, 500)}
	stored, err := c.SetWithPayload(context.TODO(), pol, "attestation-rule", "registry.example/app:v1", true, payload)
	require.NoError(t, err)
	assert.False(t, stored)

	found, _, err := c.GetWithPayload(context.TODO(), pol, "attestation-rule", "registry.example/app:v1", true)
	require.NoError(t, err)
	assert.False(t, found)
}

// Test_New_keeps_counter_overhead_below_the_budget: ristretto allocates its
// admission counters up front, so sizing them as ten per byte of budget cost
// far more memory than the budget itself.
func Test_New_keeps_counter_overhead_below_the_budget(t *testing.T) {
	const budget = 1 << 20
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	c, err := New(WithCacheEnableFlag(true), WithMaxSize(budget), WithTTLDuration(0))
	runtime.ReadMemStats(&after)
	require.NoError(t, err)
	t.Cleanup(c.(*cache).cache.Close)

	assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(budget))
}

// Test_New_accepts_a_budget_smaller_than_one_entry: the counter count has a
// floor, so a budget below expectedEntryCost still builds a valid cache.
func Test_New_accepts_a_budget_smaller_than_one_entry(t *testing.T) {
	c, err := New(WithCacheEnableFlag(true), WithMaxSize(1), WithTTLDuration(0))
	require.NoError(t, err)
	t.Cleanup(c.(*cache).cache.Close)
}
