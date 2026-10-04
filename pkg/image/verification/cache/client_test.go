package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testDigestA       = "example.com/repo@sha256:0000000000000000000000000000000000000000000000000000000000000001"
	testDigestB       = "example.com/repo@sha256:0000000000000000000000000000000000000000000000000000000000000002"
	testTagRef        = "example.com/repo:latest"
	testTagPlusDigest = "example.com/repo:latest@sha256:0000000000000000000000000000000000000000000000000000000000000001"
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

	stored, err := c.Set(context.TODO(), pol, "signature-rule", testDigestA, true)
	assert.NoError(t, err)
	assert.True(t, stored)

	found, err := c.Get(context.TODO(), pol, "signature-rule", testDigestA, true)
	assert.NoError(t, err)
	assert.True(t, found)
}

// Test_SetWithPayload_round_trips_real_payload_size confirms a payload-carrying
// entry (the attestation-cache path) round-trips correctly through the cache
// once cost accounting reflects its real size, not just that storage still
// works with the old flat cost.
func Test_SetWithPayload_round_trips_real_payload_size(t *testing.T) {
	// MaxSize must comfortably exceed the test payload's cost (1 + byte
	// length, see payloadCost): the default MaxSize of 1000 is sized for the
	// old flat per-entry cost of 1 and would reject a single real
	// multi-KB attestation payload outright.
	c, err := New(WithCacheEnableFlag(true), WithMaxSize(1_000_000), WithTTLDuration(0))
	assert.NoError(t, err)

	pol := &metav1.ObjectMeta{Name: "cost-test-policy", UID: "cost-test-uid", ResourceVersion: "1"}
	largePayload := map[string][]byte{"https://slsa.dev/provenance/v1": make([]byte, 4096)}

	stored, err := c.SetWithPayload(context.TODO(), pol, "attestation-rule", testDigestB, true, largePayload)
	assert.NoError(t, err)
	assert.True(t, stored)

	found, got, err := c.GetWithPayload(context.TODO(), pol, "attestation-rule", testDigestB, true)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, largePayload, got)
}

// Test_isDigestRef pins the three reference forms the cache guard depends on:
// a bare tag is rejected (it is registry-movable), while both a plain digest
// reference and a tag-plus-digest reference are accepted (both are pinned).
func Test_isDigestRef(t *testing.T) {
	assert.False(t, isDigestRef(testTagRef), "a bare tag must not be treated as an immutable reference")
	assert.True(t, isDigestRef(testDigestA), "a plain digest reference must be treated as immutable")
	assert.True(t, isDigestRef(testTagPlusDigest), "a tag-plus-digest reference must be treated as immutable")
}

// Test_Cache_RefusesMutableKeys is the A4-class regression: a verdict must
// never be stored or served under a mutable (tag-only) reference, in either
// direction, for both the presence-only and payload-carrying paths.
func Test_Cache_RefusesMutableKeys(t *testing.T) {
	c, err := New(WithCacheEnableFlag(true), WithMaxSize(0), WithTTLDuration(0))
	assert.NoError(t, err)

	pol := &metav1.ObjectMeta{Name: "guard-test-policy", UID: "guard-test-uid", ResourceVersion: "1"}

	t.Run("Set/Get refuse a bare tag", func(t *testing.T) {
		stored, err := c.Set(context.TODO(), pol, "rule", testTagRef, true)
		assert.NoError(t, err)
		assert.False(t, stored, "a tag-only reference must not be stored")

		found, err := c.Get(context.TODO(), pol, "rule", testTagRef, true)
		assert.NoError(t, err)
		assert.False(t, found, "a tag-only reference must never be served, even if something else wrote it")
	})

	t.Run("Set/Get accept a digest and distinguish digests", func(t *testing.T) {
		stored, err := c.Set(context.TODO(), pol, "rule", testDigestA, true)
		assert.NoError(t, err)
		assert.True(t, stored)

		found, err := c.Get(context.TODO(), pol, "rule", testDigestA, true)
		assert.NoError(t, err)
		assert.True(t, found)

		found, err = c.Get(context.TODO(), pol, "rule", testTagRef, true)
		assert.NoError(t, err)
		assert.False(t, found, "a hit for one reference form must not leak to the bare tag")

		found, err = c.Get(context.TODO(), pol, "rule", testDigestB, true)
		assert.NoError(t, err)
		assert.False(t, found, "a different digest must never hit")
	})

	t.Run("SetWithPayload/GetWithPayload refuse a bare tag", func(t *testing.T) {
		payload := map[string][]byte{"predicate": []byte("data")}
		stored, err := c.SetWithPayload(context.TODO(), pol, "attestation-rule", testTagRef, true, payload)
		assert.NoError(t, err)
		assert.False(t, stored, "a tag-only reference must not be stored")

		found, _, err := c.GetWithPayload(context.TODO(), pol, "attestation-rule", testTagRef, true)
		assert.NoError(t, err)
		assert.False(t, found, "a tag-only reference must never be served")
	})
}
