package cosign

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/cosign/v3/pkg/oci/mutate"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	ocistatic "github.com/sigstore/cosign/v3/pkg/oci/static"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	sgbundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/sign"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/dsse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

// signV3Bundle builds a key-signed v0.3 sigstore bundle wrapping an in-toto
// statement whose single subject digest is subjectDigestHex, with no
// transparency-log entry or timestamp (the shape a key attestor with the log
// ignored accepts), and returns the bundle JSON and the PEM public key that
// verifies it.
func signV3Bundle(t *testing.T, subjectDigestHex, predicateType string) ([]byte, string) {
	t.Helper()
	kp, err := sign.NewEphemeralKeypair(nil)
	require.NoError(t, err)

	statement := fmt.Sprintf(
		`{"_type":"https://in-toto.io/Statement/v1","predicateType":%q,"subject":[{"name":"subject","digest":{"sha256":%q}}],"predicate":{}}`,
		predicateType, subjectDigestHex,
	)
	pb, err := sign.Bundle(&sign.DSSEData{Data: []byte(statement), PayloadType: "application/vnd.in-toto+json"}, kp, sign.BundleOptions{})
	require.NoError(t, err)

	raw, err := protojson.Marshal(pb)
	require.NoError(t, err)

	pubPEM, err := kp.GetPublicKeyPem()
	require.NoError(t, err)

	return raw, pubPEM
}

// otherPublicKey generates a second keypair and returns only its PEM, for
// tests that need an attestor key that does not match the bundle's signer.
func otherPublicKey(t *testing.T) string {
	t.Helper()
	kp, err := sign.NewEphemeralKeypair(nil)
	require.NoError(t, err)
	pubPEM, err := kp.GetPublicKeyPem()
	require.NoError(t, err)
	return pubPEM
}

// Test_V3Bundle_GenuineKeySigned_Verifies is the positive control: a genuine
// key-signed v0.3 bundle must verify end to end through VerifyImageSignature.
// This proves the per-bundle wrapper's artifact-digest policy and claim
// verifier are both wired correctly; the two negative variants below keep it
// falsifiable.
func Test_V3Bundle_GenuineKeySigned_Verifies(t *testing.T) {
	repo, _ := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	img := fetchTestImage(t, repo.Tag("subject").String())

	bundleJSON, pubPEM := signV3Bundle(t, subject.Digest.Hex, "https://example.com/predicate")
	pushReferrer(t, repo, subject, bundleJSON, testBundleMediaType, testBundleMediaType)

	v := NewVerifier(nil, logr.Discard())
	attestor := &v1beta1.Attestor{Name: "test", Cosign: keyAttestorFor(pubPEM)}
	require.NoError(t, v.VerifyImageSignature(context.TODO(), img, attestor))
}

// Test_V3Bundle_StatementSubjectMismatch_Fails pins the artifact-digest policy:
// a bundle whose statement subject names a different digest must not verify.
func Test_V3Bundle_StatementSubjectMismatch_Fails(t *testing.T) {
	repo, _ := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	img := fetchTestImage(t, repo.Tag("subject").String())

	wrongDigest := strings.Repeat("0", 64)
	bundleJSON, pubPEM := signV3Bundle(t, wrongDigest, "https://example.com/predicate")
	pushReferrer(t, repo, subject, bundleJSON, testBundleMediaType, testBundleMediaType)

	v := NewVerifier(nil, logr.Discard())
	attestor := &v1beta1.Attestor{Name: "test", Cosign: keyAttestorFor(pubPEM)}
	err := v.VerifyImageSignature(context.TODO(), img, attestor)
	require.Error(t, err)
}

// Test_V3Bundle_WrongKey_Fails pins the signature check itself: an attestor
// configured with a different public key than the one that signed the bundle
// must not verify.
func Test_V3Bundle_WrongKey_Fails(t *testing.T) {
	repo, _ := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	img := fetchTestImage(t, repo.Tag("subject").String())

	bundleJSON, _ := signV3Bundle(t, subject.Digest.Hex, "https://example.com/predicate")
	pushReferrer(t, repo, subject, bundleJSON, testBundleMediaType, testBundleMediaType)

	v := NewVerifier(nil, logr.Discard())
	attestor := &v1beta1.Attestor{Name: "test", Cosign: keyAttestorFor(otherPublicKey(t))}
	err := v.VerifyImageSignature(context.TODO(), img, attestor)
	require.Error(t, err)
}

// Test_V3Bundle_OversizedDecoy_IsSkipped is the sink: a genuine bundle plus an
// oversized decoy referrer must still verify on the genuine one, and the
// decoy must never be read past the bound.
func Test_V3Bundle_OversizedDecoy_IsSkipped(t *testing.T) {
	repo, blobGets := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	img := fetchTestImage(t, repo.Tag("subject").String())

	bundleJSON, pubPEM := signV3Bundle(t, subject.Digest.Hex, "https://example.com/predicate")
	pushReferrer(t, repo, subject, bundleJSON, testBundleMediaType, testBundleMediaType)
	pushReferrer(t, repo, subject, gzipPaddedBundle(t, 64<<20), testBundleMediaType, testBundleMediaType)

	v := NewVerifier(nil, logr.Discard())
	attestor := &v1beta1.Attestor{Name: "test", Cosign: keyAttestorFor(pubPEM)}

	blobGets.Store(0)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	err := v.VerifyImageSignature(context.TODO(), img, attestor)
	runtime.ReadMemStats(&after)

	alloc := after.TotalAlloc - before.TotalAlloc
	t.Logf("verification allocated %d bytes over %d blob GETs", alloc, blobGets.Load())

	require.NoError(t, err, "a genuine bundle must still verify beside an oversized decoy")
	assert.Equal(t, int64(2), blobGets.Load(), "each candidate referrer is read exactly once")
	assert.Less(t, alloc, uint64(64<<20), "the oversized decoy must never be read past the bound")
}

// Test_V3Bundle_OversizedDecoy_AttestationPathSkipsIt exists so the second
// call site (VerifyAttestationSignature) cannot regress the same way: it must
// not inflate the oversized decoy either, regardless of the outcome.
func Test_V3Bundle_OversizedDecoy_AttestationPathSkipsIt(t *testing.T) {
	repo, blobGets := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	img := fetchTestImage(t, repo.Tag("subject").String())

	predicateType := "https://example.com/predicate"
	bundleJSON, pubPEM := signV3Bundle(t, subject.Digest.Hex, predicateType)
	pushReferrer(t, repo, subject, bundleJSON, testBundleMediaType, testBundleMediaType)
	pushReferrer(t, repo, subject, gzipPaddedBundle(t, 64<<20), testBundleMediaType, testBundleMediaType)

	v := NewVerifier(nil, logr.Discard())
	attestor := &v1beta1.Attestor{Name: "test", Cosign: keyAttestorFor(pubPEM)}
	attestation := &v1beta1.Attestation{Name: "test", InToto: &v1beta1.InToto{Type: predicateType}}

	blobGets.Store(0)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	err := v.VerifyAttestationSignature(context.TODO(), img, attestation, attestor)
	runtime.ReadMemStats(&after)

	alloc := after.TotalAlloc - before.TotalAlloc
	t.Logf("verification allocated %d bytes over %d blob GETs", alloc, blobGets.Load())

	assert.NoError(t, err, "a genuine attestation bundle must still verify beside an oversized decoy")
	assert.Equal(t, int64(2), blobGets.Load(), "each candidate referrer is read exactly once")
	assert.Less(t, alloc, uint64(64<<20), "the oversized decoy must never be read past the bound")
}

// Test_V3Bundle_JunkDecoy_IsSkipped pins the rule that an unparseable referrer
// is skipped, not fatal, on the v3 path, matching cosign's own GetBundles.
func Test_V3Bundle_JunkDecoy_IsSkipped(t *testing.T) {
	repo, _ := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	img := fetchTestImage(t, repo.Tag("subject").String())

	bundleJSON, pubPEM := signV3Bundle(t, subject.Digest.Hex, "https://example.com/predicate")
	pushReferrer(t, repo, subject, bundleJSON, testBundleMediaType, testBundleMediaType)
	pushReferrer(t, repo, subject, []byte(junkBundle), testBundleMediaType, testBundleMediaType)

	v := NewVerifier(nil, logr.Discard())
	attestor := &v1beta1.Attestor{Name: "test", Cosign: keyAttestorFor(pubPEM)}
	require.NoError(t, v.VerifyImageSignature(context.TODO(), img, attestor))
}

// Test_V2SignedImage_WithOversizedBundleReferrer_IsAdmitted pins that an
// oversized referrer still falls back to the v2 path rather than denying the
// image, matching Test_V2SignedImage_WithPlantedBundleReferrer_IsAdmitted.
func Test_V2SignedImage_WithOversizedBundleReferrer_IsAdmitted(t *testing.T) {
	repo, _ := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	pubPEM := signV2(t, repo, "subject")
	pushReferrer(t, repo, subject, gzipPaddedBundle(t, 64<<20), testBundleMediaType, testBundleMediaType)

	img := fetchTestImage(t, repo.Tag("subject").String())
	v := NewVerifier(nil, logr.Discard())
	attestor := &v1beta1.Attestor{Name: "test", Cosign: keyAttestorFor(pubPEM)}

	cOpts, bundles, _, err := v.buildCheckOptsWithBundleDetection(context.TODO(), attestor.Cosign, img)
	require.NoError(t, err)
	assert.False(t, cOpts.NewBundleFormat)
	assert.Empty(t, bundles)

	require.NoError(t, v.VerifyImageSignature(context.TODO(), img, attestor),
		"a v2-signed image with an oversized bundle referrer must be admitted")
}

// Test_V3Bundle_DecoyBeforeGenuine_DoesNotChangeOptions pins the per-iteration
// CheckOpts copy: a decoy bundle verified earlier in the loop must not change
// how a later bundle is verified. The decoy here is signed by a different key
// than the attestor trusts, so it fails VerifyNewBundle before the genuine
// bundle is reached; verifyBundles is called directly with [decoy, genuine] so
// the order is guaranteed regardless of referrer digest ordering.
//
// On cosign v3.1.3 (this branch) this assertion cannot fail even without the
// per-iteration copy: VerifyNewBundle already copies CheckOpts internally
// before rekorV2Bundle runs, and rekorV2Bundle itself returns before touching
// UseSignedTimestamps whenever IgnoreTlog is set or a SigVerifier is present,
// which this attestor configuration always satisfies. The copy in
// verifyBundles is still mandatory per the design (parity with 1.17/1.18 and
// independence from a future cosign bump); this test is kept as a tripwire
// against that copy being removed in a refactor, not as the discriminator.
func Test_V3Bundle_DecoyBeforeGenuine_DoesNotChangeOptions(t *testing.T) {
	repo, _ := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	img := fetchTestImage(t, repo.Tag("subject").String())

	genuineJSON, pubPEM := signV3Bundle(t, subject.Digest.Hex, "https://example.com/predicate")
	decoyJSON, _ := signV3Bundle(t, subject.Digest.Hex, "https://example.com/predicate")

	parse := func(raw []byte) *sgbundle.Bundle {
		pb := &protobundle.Bundle{}
		require.NoError(t, protojson.Unmarshal(raw, pb))
		b, err := sgbundle.NewBundle(pb)
		require.NoError(t, err)
		return b
	}
	decoy := parse(decoyJSON)
	genuine := parse(genuineJSON)

	attestor := &v1beta1.Attestor{Name: "test", Cosign: keyAttestorFor(pubPEM)}
	cOpts, err := checkOptions(context.TODO(), attestor.Cosign, img.RemoteOpts(), img.NameOpts(), nil)
	require.NoError(t, err)
	cOpts.NewBundleFormat = true
	cOpts.ClaimVerifier = cosign.IntotoSubjectClaimVerifier

	hash := subject.Digest

	sigs, verifiedBundles, _, verified, err := verifyBundles(context.TODO(), cOpts, []*sgbundle.Bundle{decoy, genuine}, &hash)
	assert.NoError(t, err, "the genuine bundle must still verify after a decoy ahead of it fails")
	assert.True(t, verified)
	assert.False(t, cOpts.UseSignedTimestamps, "a decoy verified earlier in the loop must not change options used for a later bundle")
	assert.Len(t, sigs, 1)
	assert.Len(t, verifiedBundles, 1)
	assert.Equal(t, genuine, verifiedBundles[0], "only genuine bundle must be returned as verified")
}

func signV3BundleWithPredicate(t *testing.T, subjectDigestHex, predicateType, predicateJSON string) ([]byte, string) {
	t.Helper()
	kp, err := sign.NewEphemeralKeypair(nil)
	require.NoError(t, err)

	statement := fmt.Sprintf(
		`{"_type":"https://in-toto.io/Statement/v1","predicateType":%q,"subject":[{"name":"subject","digest":{"sha256":%q}}],"predicate":%s}`,
		predicateType, subjectDigestHex, predicateJSON,
	)
	pb, err := sign.Bundle(&sign.DSSEData{Data: []byte(statement), PayloadType: "application/vnd.in-toto+json"}, kp, sign.BundleOptions{})
	require.NoError(t, err)

	raw, err := protojson.Marshal(pb)
	require.NoError(t, err)

	pubPEM, err := kp.GetPublicKeyPem()
	require.NoError(t, err)

	return raw, pubPEM
}

func Test_V3Bundle_MultipleAttestors_PreservesNewestAcrossEvaluationOrder(t *testing.T) {
	repo, _ := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")

	pType := "https://example.com/multi-attestor"
	oldBundleJSON, pubOld := signV3BundleWithPredicate(t, subject.Digest.Hex, pType, `{"metadata":{"scanFinishedOn":"2026-01-01T00:00:00Z"},"version":"old"}`)
	newBundleJSON, pubNew := signV3BundleWithPredicate(t, subject.Digest.Hex, pType, `{"metadata":{"scanFinishedOn":"2026-02-01T00:00:00Z"},"version":"new"}`)

	pushReferrer(t, repo, subject, oldBundleJSON, testBundleMediaType, testBundleMediaType)
	pushReferrer(t, repo, subject, newBundleJSON, testBundleMediaType, testBundleMediaType)

	attestorOld := &v1beta1.Attestor{Name: "old-key", Cosign: keyAttestorFor(pubOld)}
	attestorNew := &v1beta1.Attestor{Name: "new-key", Cosign: keyAttestorFor(pubNew)}
	attestation := &v1beta1.Attestation{
		Name: "test-attestation",
		InToto: &v1beta1.InToto{
			Type: pType,
		},
	}

	// Case 1: Evaluate old attestor first, then new attestor.
	{
		ctx := WithCandidateStore(context.Background())
		img := fetchTestImage(t, repo.Tag("subject").String())
		v := NewVerifier(nil, logr.Discard())

		require.NoError(t, v.VerifyAttestationSignature(ctx, img, attestation, attestorOld))
		require.NoError(t, v.VerifyAttestationSignature(ctx, img, attestation, attestorNew))

		payload, err := img.GetPayload(*attestation)
		require.NoError(t, err)
		payloadMap, ok := payload.(map[string]any)
		require.True(t, ok)
		pred, ok := payloadMap["predicate"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "new", pred["version"], "newest attestation should be selected when old is evaluated before new")
	}

	// Case 2: Evaluate new attestor first, then old attestor.
	{
		ctx := WithCandidateStore(context.Background())
		img := fetchTestImage(t, repo.Tag("subject").String())
		v := NewVerifier(nil, logr.Discard())

		require.NoError(t, v.VerifyAttestationSignature(ctx, img, attestation, attestorNew))
		require.NoError(t, v.VerifyAttestationSignature(ctx, img, attestation, attestorOld))

		payload, err := img.GetPayload(*attestation)
		require.NoError(t, err)
		payloadMap, ok := payload.(map[string]any)
		require.True(t, ok)
		pred, ok := payloadMap["predicate"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "new", pred["version"], "newest attestation should not be overwritten when old is evaluated after new")
	}

	// Case 3: Request-scoped candidate store preserves newest attestation across attestors
	// even when many other images are evaluated in the same request (no arbitrary eviction).
	{
		ctx := WithCandidateStore(context.Background())
		img := fetchTestImage(t, repo.Tag("subject").String())
		v := NewVerifier(nil, logr.Discard())

		require.NoError(t, v.VerifyAttestationSignature(ctx, img, attestation, attestorNew))

		// Simulate evaluating 600 other images in the same request context
		store := CandidateStoreFromContext(ctx)
		require.NotNil(t, store)
		for i := 0; i < 600; i++ {
			fakeImg := &imagedataloader.ImageData{}
			store.Set(fakeImg, pType, verifiedCandidate{
				payload:       []byte(`{"version":"fake"}`),
				predicateType: pType,
			})
		}

		// Revisit img with old attestor: must not overwrite new because candidate was not evicted.
		require.NoError(t, v.VerifyAttestationSignature(ctx, img, attestation, attestorOld))

		payload, err := img.GetPayload(*attestation)
		require.NoError(t, err)
		payloadMap, ok := payload.(map[string]any)
		require.True(t, ok)
		pred, ok := payloadMap["predicate"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "new", pred["version"], "newest attestation should remain selected even after >512 images are evaluated")
	}
}

// attestV2 attaches a cosign v2 keyed DSSE in-toto attestation (no Rekor/tlog
// entry, mirroring `cosign attest --key ... --tlog-upload=false`) for the image
// at repo:tag and returns the PEM public key that verifies it.
func attestV2(t *testing.T, repo name.Repository, tag, subjectDigestHex, predicateType, predicateJSON string) string {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	pubDER, err := x509.MarshalPKIXPublicKey(priv.Public())
	require.NoError(t, err)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	ref, err := name.ParseReference(repo.Tag(tag).String())
	require.NoError(t, err)
	se, err := ociremote.SignedEntity(ref)
	require.NoError(t, err)

	if subjectDigestHex == "" {
		d, err := se.(interface{ Digest() (v1.Hash, error) }).Digest()
		require.NoError(t, err)
		subjectDigestHex = d.Hex
	}
	statement := fmt.Sprintf(
		`{"_type":"https://in-toto.io/Statement/v1","predicateType":%q,"subject":[{"name":"subject","digest":{"sha256":%q}}],"predicate":%s}`,
		predicateType, subjectDigestHex, predicateJSON,
	)

	signer, err := signature.LoadECDSASignerVerifier(priv, crypto.SHA256)
	require.NoError(t, err)
	wrapped := dsse.WrapSigner(signer, "application/vnd.in-toto+json")
	signedEnv, err := wrapped.SignMessage(bytes.NewReader([]byte(statement)))
	require.NoError(t, err)

	ociAtt, err := ocistatic.NewAttestation(signedEnv, ocistatic.WithLayerMediaType(types.MediaType("application/vnd.dsse.envelope.v1+json")))
	require.NoError(t, err)
	newSE, err := mutate.AttachAttestationToEntity(se, ociAtt)
	require.NoError(t, err)
	require.NoError(t, ociremote.WriteAttestations(repo, newSE))
	return string(pubPEM)
}

// Test_V2Attestation_Keyed_IgnoreTlog reproduces issue #17109: a keyed cosign v2
// attestation created without a transparency log entry (`--tlog-upload=false`)
// must verify and populate the extracted in-toto payload when
// `ctlog.insecureIgnoreTlog: true` is configured, while still failing on a
// wrong key, wrong predicate type, mismatched subject digest, or when
// `insecureIgnoreTlog` is false.
func Test_V2Attestation_Keyed_IgnoreTlog(t *testing.T) {
	const predicateType = "https://cyclonedx.org/bom"

	t.Run("valid keyed v2 attestation verifies with insecureIgnoreTlog", func(t *testing.T) {
		repo, _ := newBundleRegistry(t)
		pushSubject(t, repo, "subject")
		pubPEM := attestV2(t, repo, "subject", "", predicateType, `{"bomFormat":"CycloneDX"}`)

		img := fetchTestImage(t, repo.Tag("subject").String())
		v := NewVerifier(nil, logr.Discard())
		attestor := &v1beta1.Attestor{Name: "keyed", Cosign: keyAttestorFor(pubPEM)}
		attestation := &v1beta1.Attestation{Name: "sbom", InToto: &v1beta1.InToto{Type: predicateType}}

		require.NoError(t, v.VerifyAttestationSignature(context.Background(), img, attestation, attestor))

		got, err := img.GetPayload(*attestation)
		require.NoError(t, err)
		payloadMap, ok := got.(map[string]any)
		require.True(t, ok)
		pred, ok := payloadMap["predicate"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "CycloneDX", pred["bomFormat"])
	})

	t.Run("wrong key still fails with insecureIgnoreTlog", func(t *testing.T) {
		repo, _ := newBundleRegistry(t)
		pushSubject(t, repo, "subject")
		attestV2(t, repo, "subject", "", predicateType, `{"bomFormat":"CycloneDX"}`)

		img := fetchTestImage(t, repo.Tag("subject").String())
		v := NewVerifier(nil, logr.Discard())
		attestor := &v1beta1.Attestor{Name: "wrong-key", Cosign: keyAttestorFor(otherPublicKey(t))}
		attestation := &v1beta1.Attestation{Name: "sbom", InToto: &v1beta1.InToto{Type: predicateType}}

		require.Error(t, v.VerifyAttestationSignature(context.Background(), img, attestation, attestor))
	})

	t.Run("wrong predicate type still fails with insecureIgnoreTlog", func(t *testing.T) {
		repo, _ := newBundleRegistry(t)
		pushSubject(t, repo, "subject")
		pubPEM := attestV2(t, repo, "subject", "", predicateType, `{"bomFormat":"CycloneDX"}`)

		img := fetchTestImage(t, repo.Tag("subject").String())
		v := NewVerifier(nil, logr.Discard())
		attestor := &v1beta1.Attestor{Name: "keyed", Cosign: keyAttestorFor(pubPEM)}
		attestation := &v1beta1.Attestation{Name: "other", InToto: &v1beta1.InToto{Type: "https://slsa.dev/provenance/v1"}}

		require.Error(t, v.VerifyAttestationSignature(context.Background(), img, attestation, attestor))
	})

	t.Run("subject digest mismatch still fails with insecureIgnoreTlog", func(t *testing.T) {
		repo, _ := newBundleRegistry(t)
		pushSubject(t, repo, "subject")
		wrongDigest := strings.Repeat("0", 64)
		pubPEM := attestV2(t, repo, "subject", wrongDigest, predicateType, `{"bomFormat":"CycloneDX"}`)

		img := fetchTestImage(t, repo.Tag("subject").String())
		v := NewVerifier(nil, logr.Discard())
		attestor := &v1beta1.Attestor{Name: "keyed", Cosign: keyAttestorFor(pubPEM)}
		attestation := &v1beta1.Attestation{Name: "sbom", InToto: &v1beta1.InToto{Type: predicateType}}

		require.Error(t, v.VerifyAttestationSignature(context.Background(), img, attestation, attestor))
	})
}
