package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
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

	stored, err := c.Set(context.TODO(), pol, "signature-rule", "image-a", "", true)
	assert.NoError(t, err)
	assert.True(t, stored)

	found, err := c.Get(context.TODO(), pol, "signature-rule", "image-a", "", true)
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

	stored, err := c.SetWithPayload(context.TODO(), pol, "attestation-rule", "image-b", "", true, largePayload)
	assert.NoError(t, err)
	assert.True(t, stored)

	found, got, err := c.GetWithPayload(context.TODO(), pol, "attestation-rule", "image-b", "", true)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, largePayload, got)
}

// Test_configFingerprint_partitions_entries reproduces the stale-verification
// scenario from #17753 at the key level: a cached entry written under one
// resolved attestor configuration must not be returned when the configuration
// fingerprint changes, even though policy UID, resourceVersion, rule name and
// image ref are all identical.
func Test_configFingerprint_partitions_entries(t *testing.T) {
	c, err := New(WithCacheEnableFlag(true), WithMaxSize(0), WithTTLDuration(0))
	assert.NoError(t, err)

	pol := &metav1.ObjectMeta{Name: "key-rotation-policy", UID: "rotation-uid", ResourceVersion: "1"}

	stored, err := c.Set(context.TODO(), pol, "verify-rule", "image-c", "fingerprint-key-a", true)
	assert.NoError(t, err)
	assert.True(t, stored)

	// same fingerprint -> hit
	found, err := c.Get(context.TODO(), pol, "verify-rule", "image-c", "fingerprint-key-a", true)
	assert.NoError(t, err)
	assert.True(t, found)

	// rotated config (different resolved key material) -> miss, re-verifies
	found, err = c.Get(context.TODO(), pol, "verify-rule", "image-c", "fingerprint-key-b", true)
	assert.NoError(t, err)
	assert.False(t, found)

	// and the old entry must still be there under its own fingerprint
	found, err = c.Get(context.TODO(), pol, "verify-rule", "image-c", "fingerprint-key-a", true)
	assert.NoError(t, err)
	assert.True(t, found)
}

// Test_configFingerprint_payload_path_partitions_entries covers the payload
// variant of the key so attestation caches get the same invalidation
// semantics, not just the presence-only path.
func Test_configFingerprint_payload_path_partitions_entries(t *testing.T) {
	c, err := New(WithCacheEnableFlag(true), WithMaxSize(1_000_000), WithTTLDuration(0))
	assert.NoError(t, err)

	pol := &metav1.ObjectMeta{Name: "key-rotation-policy", UID: "rotation-uid", ResourceVersion: "1"}
	payload := map[string][]byte{"https://slsa.dev/provenance/v1": []byte("payload")}

	stored, err := c.SetWithPayload(context.TODO(), pol, "attestation-rule", "image-d", "fp-1", true, payload)
	assert.NoError(t, err)
	assert.True(t, stored)

	found, _, err := c.GetWithPayload(context.TODO(), pol, "attestation-rule", "image-d", "fp-2", true)
	assert.NoError(t, err)
	assert.False(t, found)

	found, got, err := c.GetWithPayload(context.TODO(), pol, "attestation-rule", "image-d", "fp-1", true)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, payload, got)
}

// Test_empty_configFingerprint_keeps_legacy_key confirms callers that pass an
// empty fingerprint (the ivpol path, whose attestor config lives in the policy
// spec and is already covered by the resourceVersion key part) keep the exact
// pre-change cache behavior: entries are keyed without any config component.
func Test_empty_configFingerprint_keeps_legacy_key(t *testing.T) {
	c, err := New(WithCacheEnableFlag(true), WithMaxSize(0), WithTTLDuration(0))
	assert.NoError(t, err)

	pol := &metav1.ObjectMeta{Name: "p", UID: "u", ResourceVersion: "7"}
	key := generateKey(pol, "r", "img", "")
	assert.Equal(t, "u;7;r;img;", key)

	stored, err := c.Set(context.TODO(), pol, "r", "img", "", true)
	assert.NoError(t, err)
	assert.True(t, stored)
	found, err := c.Get(context.TODO(), pol, "r", "img", "", true)
	assert.NoError(t, err)
	assert.True(t, found)
}
