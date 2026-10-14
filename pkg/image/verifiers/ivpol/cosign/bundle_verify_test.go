package cosign

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	sgbundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/sign"
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

	_, verified, err := verifyBundles(context.TODO(), cOpts, []*sgbundle.Bundle{decoy, genuine}, &hash)
	assert.NoError(t, err, "the genuine bundle must still verify after a decoy ahead of it fails")
	assert.True(t, verified)
	assert.False(t, cOpts.UseSignedTimestamps, "a decoy verified earlier in the loop must not change options used for a later bundle")
}
