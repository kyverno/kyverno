package cache

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/go-logr/logr/funcr"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testDigestA       = "example.com/repo@sha256:0000000000000000000000000000000000000000000000000000000000000001"
	testDigestB       = "example.com/repo@sha256:0000000000000000000000000000000000000000000000000000000000000002"
	testTagRef        = "example.com/repo:latest"
	testTagPlusDigest = "example.com/repo:latest@sha256:0000000000000000000000000000000000000000000000000000000000000001"
)

func keyAttestor(name, key string) v1beta1.Attestor {
	return v1beta1.Attestor{Name: name, Cosign: &v1beta1.Cosign{Key: &v1beta1.Key{Data: key}}}
}

var (
	testAttestors   = []v1beta1.Attestor{keyAttestor("signer", "KEY-A")}
	testAttestation = &v1beta1.Attestation{Name: "provenance", InToto: &v1beta1.InToto{Type: "https://slsa.dev/provenance/v1"}}
)

func newTestCache(t *testing.T, opts ...Option) Client {
	t.Helper()
	c, err := New(append([]Option{WithMaxSize(0), WithTTLDuration(0)}, opts...)...)
	require.NoError(t, err)
	return c
}

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

// Test_Set_signature_entry_round_trips confirms a signature entry (no
// attestation, nil payload) is stored and served.
func Test_Set_signature_entry_round_trips(t *testing.T) {
	c := newTestCache(t)

	stored, err := c.Set(testDigestA, testAttestors, nil, nil)
	assert.NoError(t, err)
	assert.True(t, stored)

	found, payloads, err := c.Get(testDigestA, testAttestors, nil)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Nil(t, payloads)
}

// Test_Set_round_trips_real_payload_size confirms a payload-carrying entry (the
// attestation-cache path) round-trips correctly once cost accounting reflects
// its real size.
func Test_Set_round_trips_real_payload_size(t *testing.T) {
	// MaxSize must comfortably exceed the test payload's cost (1 + byte
	// length, see payloadCost).
	c := newTestCache(t, WithMaxSize(1_000_000))
	largePayload := map[string][]byte{"https://slsa.dev/provenance/v1": make([]byte, 4096)}

	stored, err := c.Set(testDigestB, testAttestors, testAttestation, largePayload)
	assert.NoError(t, err)
	assert.True(t, stored)

	found, got, err := c.Get(testDigestB, testAttestors, testAttestation)
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
// direction, for both signature and attestation entries.
func Test_Cache_RefusesMutableKeys(t *testing.T) {
	c := newTestCache(t)

	t.Run("Set/Get refuse a bare tag", func(t *testing.T) {
		stored, err := c.Set(testTagRef, testAttestors, nil, nil)
		assert.NoError(t, err)
		assert.False(t, stored, "a tag-only reference must not be stored")

		found, _, err := c.Get(testTagRef, testAttestors, nil)
		assert.NoError(t, err)
		assert.False(t, found, "a tag-only reference must never be served, even if something else wrote it")
	})

	t.Run("Get refuses a bare-tag entry already in the store", func(t *testing.T) {
		// Set refuses the key, so the sub-test above cannot tell the read guard
		// apart from an ordinary miss. Seed the store directly to exercise the
		// read guard on its own.
		impl, ok := c.(*cache)
		assert.True(t, ok)
		key, err := generateKey(testTagRef, testAttestors, nil)
		require.NoError(t, err)
		impl.cache.SetWithTTL(key, nil, 1, time.Hour)
		impl.cache.Wait()
		// Without this, a store that dropped the entry would make the
		// assertions below pass as ordinary misses. Ristretto reports presence
		// separately from the stored value, which is nil here.
		_, seeded := impl.cache.Get(key)
		assert.True(t, seeded, "the entry must actually be in the store for this to test the read guard")

		found, payloads, err := c.Get(testTagRef, testAttestors, nil)
		assert.NoError(t, err)
		assert.False(t, found, "a seeded tag-only entry must still never be served")
		assert.Nil(t, payloads)
	})

	t.Run("Set/Get accept a digest and distinguish digests", func(t *testing.T) {
		stored, err := c.Set(testDigestA, testAttestors, nil, nil)
		assert.NoError(t, err)
		assert.True(t, stored)

		found, _, err := c.Get(testDigestA, testAttestors, nil)
		assert.NoError(t, err)
		assert.True(t, found)

		found, _, err = c.Get(testTagRef, testAttestors, nil)
		assert.NoError(t, err)
		assert.False(t, found, "a hit for one reference form must not leak to the bare tag")

		found, _, err = c.Get(testDigestB, testAttestors, nil)
		assert.NoError(t, err)
		assert.False(t, found, "a different digest must never hit")
	})

	t.Run("attestation entries refuse a bare tag", func(t *testing.T) {
		payload := map[string][]byte{"predicate": []byte("data")}
		stored, err := c.Set(testTagRef, testAttestors, testAttestation, payload)
		assert.NoError(t, err)
		assert.False(t, stored, "a tag-only reference must not be stored")

		found, _, err := c.Get(testTagRef, testAttestors, testAttestation)
		assert.NoError(t, err)
		assert.False(t, found, "a tag-only reference must never be served")
	})
}

// Test_generateKey_binds_material_not_names pins that the key tracks the
// resolved attestor material and the attestation definition, not the labels a
// policy author picked, and that the attestor order does not matter.
func Test_generateKey_binds_material_not_names(t *testing.T) {
	key := func(attestors []v1beta1.Attestor, attestation *v1beta1.Attestation) string {
		t.Helper()
		k, err := generateKey(testDigestA, attestors, attestation)
		require.NoError(t, err)
		return k
	}
	a, b := keyAttestor("signer", "KEY-A"), keyAttestor("signer", "KEY-B")

	assert.NotEqual(t, key([]v1beta1.Attestor{a}, nil), key([]v1beta1.Attestor{b}, nil), "different key material under the same attestor name must not share an entry")
	assert.Equal(t, key([]v1beta1.Attestor{a}, nil), key([]v1beta1.Attestor{a}, nil), "the same material must produce the same key")
	assert.Equal(t, key([]v1beta1.Attestor{a, b}, nil), key([]v1beta1.Attestor{b, a}, nil), "attestor order must not matter")
	assert.Equal(t, key([]v1beta1.Attestor{a}, nil), key([]v1beta1.Attestor{keyAttestor("another-policys-name", "KEY-A")}, nil), "the attestor name is a label and must not change the key")

	renamed := *testAttestation
	renamed.Name = "slsa"
	assert.Equal(t, key(testAttestors, testAttestation), key(testAttestors, &renamed), "the attestation name is a label and must not change the key")

	otherType := *testAttestation
	otherType.InToto = &v1beta1.InToto{Type: "https://cyclonedx.org/bom"}
	assert.NotEqual(t, key(testAttestors, testAttestation), key(testAttestors, &otherType), "a different predicate type must not share an entry")

	assert.NotEqual(t, key(testAttestors, nil), key(testAttestors, testAttestation), "a signature entry must never share a key with an attestation entry")
}

// Test_Cache_shares_entries_across_callers confirms entries are keyed by what
// was verified: any caller with the same attestors and attestation definition
// gets the hit.
func Test_Cache_shares_entries_across_callers(t *testing.T) {
	c := newTestCache(t)
	payload := map[string][]byte{"https://slsa.dev/provenance/v1": []byte("{}")}

	stored, err := c.Set(testDigestA, testAttestors, testAttestation, payload)
	require.NoError(t, err)
	require.True(t, stored)

	renamed := *testAttestation
	renamed.Name = "another-policys-name"
	found, got, err := c.Get(testDigestA, testAttestors, &renamed)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, payload, got)

	found, _, err = c.Get(testDigestA, testAttestors, nil)
	require.NoError(t, err)
	assert.False(t, found, "an attestation entry must not satisfy a signature lookup")
}

// Test_Cache_shares_entries_for_equivalent_digest_references verifies that
// references pinned to the same immutable digest (with or without tag components)
// canonicalize to the same cache key and share cache hits.
func Test_Cache_shares_entries_for_equivalent_digest_references(t *testing.T) {
	c := newTestCache(t)
	payload := map[string][]byte{"https://slsa.dev/provenance/v1": []byte("{}")}

	refWithTag := "ghcr.io/example/app:v1@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105"
	refWithoutTag := "ghcr.io/example/app@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105"

	stored, err := c.Set(refWithTag, testAttestors, testAttestation, payload)
	require.NoError(t, err)
	require.True(t, stored)

	found, got, err := c.Get(refWithoutTag, testAttestors, testAttestation)
	require.NoError(t, err)
	assert.True(t, found, "references to the same digest (with and without tag) must share cache entries")
	assert.Equal(t, payload, got)

	// In reverse: store without tag, read with tag
	stored, err = c.Set(testDigestA, testAttestors, nil, nil)
	require.NoError(t, err)
	require.True(t, stored)

	found, got, err = c.Get(testTagPlusDigest, testAttestors, nil)
	require.NoError(t, err)
	assert.True(t, found, "storing under untagged digest must hit when querying with tag+digest")
	assert.Nil(t, got)
}

// Test_generateKey_binds_resolved_material_not_expressions verifies that
// attestors with identical resolved key material produce identical cache keys,
// even when one or both were resolved dynamically via CEL expressions.
// It also verifies that differing resolved trust materials produce differing
// cache keys and cache misses even when dynamic expressions are unchanged.
func Test_generateKey_binds_resolved_material_not_expressions(t *testing.T) {
	t.Run("cosign key expressions", func(t *testing.T) {
		inline := keyAttestor("signer", "KEY-MATERIAL")
		withExpr1 := keyAttestor("signer", "KEY-MATERIAL")
		withExpr1.Cosign.Key.Expression = "variables.trustedKey"
		withExpr2 := keyAttestor("signer", "KEY-MATERIAL")
		withExpr2.Cosign.Key.Expression = "variables.otherKey"

		k1, err := generateKey(testDigestA, []v1beta1.Attestor{inline}, nil)
		require.NoError(t, err)
		k2, err := generateKey(testDigestA, []v1beta1.Attestor{withExpr1}, nil)
		require.NoError(t, err)
		k3, err := generateKey(testDigestA, []v1beta1.Attestor{withExpr2}, nil)
		require.NoError(t, err)

		assert.Equal(t, k1, k2, "static and CEL-resolved key material must produce identical cache keys")
		assert.Equal(t, k2, k3, "differing CEL expressions resolving to the same key material must produce identical cache keys")

		// Negative check: changing resolved key material with expression unchanged produces different key and misses cache
		withDiffKey := keyAttestor("signer", "DIFFERENT-KEY-MATERIAL")
		withDiffKey.Cosign.Key.Expression = "variables.trustedKey"
		kDiff, err := generateKey(testDigestA, []v1beta1.Attestor{withDiffKey}, nil)
		require.NoError(t, err)
		assert.NotEqual(t, k1, kDiff, "differing key material must produce differing cache keys")

		c := newTestCache(t)
		stored, err := c.Set(testDigestA, []v1beta1.Attestor{withExpr1}, nil, nil)
		require.NoError(t, err)
		require.True(t, stored)
		found, _, err := c.Get(testDigestA, []v1beta1.Attestor{withDiffKey}, nil)
		require.NoError(t, err)
		assert.False(t, found, "differing key material must miss cache")
	})

	t.Run("cosign certificate and chain expressions", func(t *testing.T) {
		certStatic := v1beta1.Attestor{
			Cosign: &v1beta1.Cosign{
				Certificate: &v1beta1.Certificate{
					Certificate:      &v1beta1.StringOrExpression{Value: "CERT-PEM"},
					CertificateChain: &v1beta1.StringOrExpression{Value: "CHAIN-PEM"},
				},
			},
		}
		certWithExpr := v1beta1.Attestor{
			Cosign: &v1beta1.Cosign{
				Certificate: &v1beta1.Certificate{
					Certificate:      &v1beta1.StringOrExpression{Value: "CERT-PEM", Expression: "variables.cert"},
					CertificateChain: &v1beta1.StringOrExpression{Value: "CHAIN-PEM", Expression: "variables.chain"},
				},
			},
		}

		k1, err := generateKey(testDigestA, []v1beta1.Attestor{certStatic}, nil)
		require.NoError(t, err)
		k2, err := generateKey(testDigestA, []v1beta1.Attestor{certWithExpr}, nil)
		require.NoError(t, err)

		assert.Equal(t, k1, k2, "certificate and chain expressions must be cleared so identical material matches")

		// Negative checks: changing certificate or chain material independently while keeping expressions unchanged
		certDiffCert := *certWithExpr.DeepCopy()
		certDiffCert.Cosign.Certificate.Certificate.Value = "DIFFERENT-CERT-PEM"
		kDiffCert, err := generateKey(testDigestA, []v1beta1.Attestor{certDiffCert}, nil)
		require.NoError(t, err)
		assert.NotEqual(t, k1, kDiffCert, "differing certificate value must produce differing cache keys")

		certDiffChain := *certWithExpr.DeepCopy()
		certDiffChain.Cosign.Certificate.CertificateChain.Value = "DIFFERENT-CHAIN-PEM"
		kDiffChain, err := generateKey(testDigestA, []v1beta1.Attestor{certDiffChain}, nil)
		require.NoError(t, err)
		assert.NotEqual(t, k1, kDiffChain, "differing certificate chain value must produce differing cache keys")

		c := newTestCache(t)
		stored, err := c.Set(testDigestA, []v1beta1.Attestor{certWithExpr}, nil, nil)
		require.NoError(t, err)
		require.True(t, stored)

		found, _, err := c.Get(testDigestA, []v1beta1.Attestor{certDiffCert}, nil)
		require.NoError(t, err)
		assert.False(t, found, "differing certificate value must miss cache")

		found, _, err = c.Get(testDigestA, []v1beta1.Attestor{certDiffChain}, nil)
		require.NoError(t, err)
		assert.False(t, found, "differing certificate chain value must miss cache")
	})

	t.Run("cosign trusted root expressions", func(t *testing.T) {
		rootStatic := v1beta1.Attestor{
			Cosign: &v1beta1.Cosign{
				TrustedRoot: &v1beta1.StringOrExpression{Value: "ROOT-PEM"},
			},
		}
		rootWithExpr := v1beta1.Attestor{
			Cosign: &v1beta1.Cosign{
				TrustedRoot: &v1beta1.StringOrExpression{Value: "ROOT-PEM", Expression: "variables.root"},
			},
		}

		k1, err := generateKey(testDigestA, []v1beta1.Attestor{rootStatic}, nil)
		require.NoError(t, err)
		k2, err := generateKey(testDigestA, []v1beta1.Attestor{rootWithExpr}, nil)
		require.NoError(t, err)

		assert.Equal(t, k1, k2, "trusted root expressions must be cleared")

		// Negative check: changing trusted root material while keeping expression unchanged
		rootDiff := *rootWithExpr.DeepCopy()
		rootDiff.Cosign.TrustedRoot.Value = "DIFFERENT-ROOT-PEM"
		kDiffRoot, err := generateKey(testDigestA, []v1beta1.Attestor{rootDiff}, nil)
		require.NoError(t, err)
		assert.NotEqual(t, k1, kDiffRoot, "differing trusted root value must produce differing cache keys")

		c := newTestCache(t)
		stored, err := c.Set(testDigestA, []v1beta1.Attestor{rootWithExpr}, nil, nil)
		require.NoError(t, err)
		require.True(t, stored)

		found, _, err := c.Get(testDigestA, []v1beta1.Attestor{rootDiff}, nil)
		require.NoError(t, err)
		assert.False(t, found, "differing trusted root value must miss cache")
	})

	t.Run("notary certs and TSA cert expressions", func(t *testing.T) {
		notaryStatic := v1beta1.Attestor{
			Notary: &v1beta1.Notary{
				Certs:    &v1beta1.StringOrExpression{Value: "NOTARY-CERT"},
				TSACerts: &v1beta1.StringOrExpression{Value: "TSA-CERT"},
			},
		}
		notaryWithExpr := v1beta1.Attestor{
			Notary: &v1beta1.Notary{
				Certs:    &v1beta1.StringOrExpression{Value: "NOTARY-CERT", Expression: "variables.nc"},
				TSACerts: &v1beta1.StringOrExpression{Value: "TSA-CERT", Expression: "variables.tsa"},
			},
		}

		k1, err := generateKey(testDigestA, []v1beta1.Attestor{notaryStatic}, nil)
		require.NoError(t, err)
		k2, err := generateKey(testDigestA, []v1beta1.Attestor{notaryWithExpr}, nil)
		require.NoError(t, err)

		assert.Equal(t, k1, k2, "notary cert expressions must be cleared")

		// Negative checks: changing Notary certs or TSA certs independently while keeping expressions unchanged
		notaryDiffCerts := *notaryWithExpr.DeepCopy()
		notaryDiffCerts.Notary.Certs.Value = "DIFFERENT-NOTARY-CERT"
		kDiffCerts, err := generateKey(testDigestA, []v1beta1.Attestor{notaryDiffCerts}, nil)
		require.NoError(t, err)
		assert.NotEqual(t, k1, kDiffCerts, "differing notary certs value must produce differing cache keys")

		notaryDiffTSA := *notaryWithExpr.DeepCopy()
		notaryDiffTSA.Notary.TSACerts.Value = "DIFFERENT-TSA-CERT"
		kDiffTSA, err := generateKey(testDigestA, []v1beta1.Attestor{notaryDiffTSA}, nil)
		require.NoError(t, err)
		assert.NotEqual(t, k1, kDiffTSA, "differing TSA certs value must produce differing cache keys")

		c := newTestCache(t)
		stored, err := c.Set(testDigestA, []v1beta1.Attestor{notaryWithExpr}, nil, nil)
		require.NoError(t, err)
		require.True(t, stored)

		found, _, err := c.Get(testDigestA, []v1beta1.Attestor{notaryDiffCerts}, nil)
		require.NoError(t, err)
		assert.False(t, found, "differing notary certs value must miss cache")

		found, _, err = c.Get(testDigestA, []v1beta1.Attestor{notaryDiffTSA}, nil)
		require.NoError(t, err)
		assert.False(t, found, "differing TSA certs value must miss cache")
	})
}

// Test_Cache_skips_uncacheable_attestors pins the attestor groups the cache
// refuses: none at all, and KMS keys whose material the key cannot bind.
func Test_Cache_skips_uncacheable_attestors(t *testing.T) {
	kms := []v1beta1.Attestor{{Name: "kms", Cosign: &v1beta1.Cosign{Key: &v1beta1.Key{KMS: "awskms:///alias/signer"}}}}
	for name, attestors := range map[string][]v1beta1.Attestor{
		"no attestors": nil,
		"kms key":      kms,
		"kms in group": append([]v1beta1.Attestor{keyAttestor("inline", "KEY-A")}, kms...),
	} {
		t.Run(name, func(t *testing.T) {
			c := newTestCache(t)
			stored, err := c.Set(testDigestA, attestors, nil, nil)
			require.NoError(t, err)
			assert.False(t, stored)

			found, _, err := c.Get(testDigestA, attestors, nil)
			require.NoError(t, err)
			assert.False(t, found)
		})
	}
	t.Run("kms entry already in the store is not served", func(t *testing.T) {
		c := newTestCache(t)
		impl, ok := c.(*cache)
		require.True(t, ok)
		key, err := generateKey(testDigestA, kms, nil)
		require.NoError(t, err)
		impl.cache.SetWithTTL(key, nil, 1, time.Hour)
		impl.cache.Wait()
		_, seeded := impl.cache.Get(key)
		require.True(t, seeded, "the entry must actually be in the store for this to test the read guard")

		found, _, err := c.Get(testDigestA, kms, nil)
		require.NoError(t, err)
		assert.False(t, found)
	})
	assert.False(t, hasUnbindableKeyMaterial([]v1beta1.Attestor{keyAttestor("inline", "KEY-A")}), "an inline key is bound by the hash")
	assert.False(t, hasUnbindableKeyMaterial([]v1beta1.Attestor{{Name: "expr", Cosign: &v1beta1.Cosign{Key: &v1beta1.Key{Expression: "variables.key"}}}}), "an expression key is materialized before hashing")
	assert.False(t, hasUnbindableKeyMaterial([]v1beta1.Attestor{{Name: "notary", Notary: &v1beta1.Notary{}}}), "a notary attestor carries no KMS key")
}

// Test_default_budget_holds_many_presence_only_entries: the flag used to
// promise 1000 keys, but ristretto also charges its own per-item cost, so a
// 1000 budget kept only about 17 entries.
func Test_default_budget_holds_many_presence_only_entries(t *testing.T) {
	c := newTestCache(t)

	const entries = 1000
	for i := range entries {
		stored, err := c.Set(fmt.Sprintf("registry.example/app@sha256:%064x", i), testAttestors, nil, nil)
		require.NoError(t, err)
		require.True(t, stored)
	}
	found := 0
	for i := range entries {
		ok, _, err := c.Get(fmt.Sprintf("registry.example/app@sha256:%064x", i), testAttestors, nil)
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
			c := newTestCache(t)

			payload := map[string][]byte{"https://slsa.dev/provenance/v1": make([]byte, size)}
			stored, err := c.Set(testDigestA, testAttestors, testAttestation, payload)
			require.NoError(t, err)
			assert.True(t, stored)

			found, got, err := c.Get(testDigestA, testAttestors, testAttestation)
			require.NoError(t, err)
			require.True(t, found)
			assert.Equal(t, payload, got)
		})
	}
}

// Test_Set_reports_an_entry_larger_than_the_budget_as_not_stored: ristretto
// accepts the write and drops it afterwards, so the result has to come from the
// cache contents, not from the write call.
func Test_Set_reports_an_entry_larger_than_the_budget_as_not_stored(t *testing.T) {
	c := newTestCache(t, WithMaxSize(100))

	payload := map[string][]byte{"https://slsa.dev/provenance/v1": make([]byte, 500)}
	stored, err := c.Set(testDigestA, testAttestors, testAttestation, payload)
	require.NoError(t, err)
	assert.False(t, stored)

	found, _, err := c.Get(testDigestA, testAttestors, testAttestation)
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
	c, err := New(WithMaxSize(budget), WithTTLDuration(0))
	runtime.ReadMemStats(&after)
	require.NoError(t, err)
	t.Cleanup(c.(*cache).cache.Close)

	assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(budget))
}

// Test_New_accepts_a_budget_smaller_than_one_entry: the counter count has a
// floor, so a budget below expectedEntryCost still builds a valid cache.
func Test_New_accepts_a_budget_smaller_than_one_entry(t *testing.T) {
	c, err := New(WithMaxSize(1), WithTTLDuration(0))
	require.NoError(t, err)
	t.Cleanup(c.(*cache).cache.Close)
}

// Test_Set_replacing_an_entry_keeps_the_budget: ristretto updates an existing
// key without checking MaxCost, so rewriting a presence-only entry (the
// attestation re-verification path) could keep a payload larger than the whole
// budget.
func Test_Set_replacing_an_entry_keeps_the_budget(t *testing.T) {
	c := newTestCache(t, WithMaxSize(100))

	stored, err := c.Set(testDigestA, testAttestors, testAttestation, nil)
	require.NoError(t, err)
	require.True(t, stored)

	payload := map[string][]byte{"https://slsa.dev/provenance/v1": make([]byte, 500)}
	stored, err = c.Set(testDigestA, testAttestors, testAttestation, payload)
	require.NoError(t, err)
	assert.False(t, stored)

	found, got, err := c.Get(testDigestA, testAttestors, testAttestation)
	require.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, got)
}

// Test_Set_replacing_an_entry_with_a_fitting_payload_stores_it: the usual
// re-verification case, where a presence-only entry gets its payload.
func Test_Set_replacing_an_entry_with_a_fitting_payload_stores_it(t *testing.T) {
	c := newTestCache(t)

	stored, err := c.Set(testDigestA, testAttestors, testAttestation, nil)
	require.NoError(t, err)
	require.True(t, stored)

	payload := map[string][]byte{"https://slsa.dev/provenance/v1": make([]byte, 1033)}
	stored, err = c.Set(testDigestA, testAttestors, testAttestation, payload)
	require.NoError(t, err)
	assert.True(t, stored)

	found, got, err := c.Get(testDigestA, testAttestors, testAttestation)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, payload, got)
}

// Test_Set_logs_an_entry_the_cache_did_not_keep: callers don't act on the
// result, so without a log an attestation that never fits is verified again on
// every admission with nothing showing why.
func Test_Set_logs_an_entry_the_cache_did_not_keep(t *testing.T) {
	var logs []string
	logger := funcr.New(func(_, args string) { logs = append(logs, args) }, funcr.Options{Verbosity: 3})
	c := newTestCache(t, WithMaxSize(100), WithLogger(logger))

	stored, err := c.Set(testDigestA, testAttestors, nil, nil)
	require.NoError(t, err)
	require.True(t, stored)
	assert.Empty(t, logs, "an entry the cache kept must not be logged")

	payload := map[string][]byte{"https://slsa.dev/provenance/v1": make([]byte, 500)}
	stored, err = c.Set(testDigestB, testAttestors, testAttestation, payload)
	require.NoError(t, err)
	require.False(t, stored)
	require.Len(t, logs, 1)
	assert.Contains(t, logs[0], testDigestB)
}
