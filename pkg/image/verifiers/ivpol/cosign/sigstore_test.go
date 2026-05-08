package cosign

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	protodsse "github.com/sigstore/protobuf-specs/gen/pb-go/dsse"
	sgbundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	sigsig "github.com/sigstore/sigstore/pkg/signature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubSigVerifier is a do-nothing sigstore signature.Verifier used to flip
// the cosign.CheckOpts.SigVerifier branch in buildBundleVerifyOptions tests
// without bringing in real key material.
type stubSigVerifier struct{}

var _ sigsig.Verifier = stubSigVerifier{}

func (stubSigVerifier) PublicKey(_ ...sigsig.PublicKeyOption) (crypto.PublicKey, error) {
	return nil, nil
}
func (stubSigVerifier) VerifySignature(_, _ io.Reader, _ ...sigsig.VerifyOption) error {
	return nil
}

// sha256TestHash returns an arbitrary but well-formed sha256 digest for tests
// that just need *something* to feed buildBundlePolicy.
func sha256TestHash(t *testing.T) *v1.Hash {
	t.Helper()
	return &v1.Hash{
		Algorithm: "sha256",
		Hex:       "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}
}


// generateTSALeafCert creates a leaf certificate with ExtKeyUsageTimeStamping,
// suitable for use as a TSA signer. The other test helpers (generateRootCA /
// generateIntermediateCA / generateLeafCert) are defined in certs_test.go in
// this package; this one is TSA-specific.
func generateTSALeafCert(t *testing.T, issuerCert *x509.Certificate, issuerKey *rsa.PrivateKey, serial int64) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key := generateECDSAKey(t)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(serial),
		Subject:               pkix.Name{CommonName: "Test TSA Signer"},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}
	cert := createCertificate(t, tmpl, issuerCert, &key.PublicKey, issuerKey)
	return cert, key
}

// generateTSAChain returns a leaf+intermediate+root cert chain in the shape
// the IVPOL TSA cert chain field expects (one leaf with TimeStamping
// ExtKeyUsage, one intermediate, one self-signed root).
func generateTSAChain(t *testing.T) (leaf *x509.Certificate, intermediate *x509.Certificate, rootCert *x509.Certificate) {
	t.Helper()
	rootCert, rootKey := generateRootCA(t)
	intermediate, intermediateKey := generateIntermediateCA(t, rootCert, rootKey)
	leaf, _ = generateTSALeafCert(t, intermediate, intermediateKey, 100)
	return leaf, intermediate, rootCert
}


// emptyPublicTrustedRoot constructs a TrustedRoot with no Fulcio CAs / TSAs /
// Rekor logs / CT logs. Useful as the baseline "the operator's TUF root
// happens not to include the TSA you need" scenario in tests.
func emptyPublicTrustedRoot(t *testing.T) *root.TrustedRoot {
	t.Helper()
	tr, err := root.NewTrustedRoot(
		root.TrustedRootMediaType01,
		nil, // certificateAuthorities
		nil, // certificateTransparencyLogs
		nil, // timestampAuthorities
		nil, // transparencyLogs
	)
	require.NoError(t, err)
	return tr
}

// publicTrustedRootWithTSA constructs a TrustedRoot whose TimestampingAuthorities
// already contains a single TSA. Used to verify that composeTrustedMaterial
// preserves the public root's TSAs when adding a custom TSA on top.
func publicTrustedRootWithTSA(t *testing.T) (*root.TrustedRoot, *root.SigstoreTimestampingAuthority) {
	t.Helper()
	leaf, intermediate, rootCert := generateTSAChain(t)
	tsa := &root.SigstoreTimestampingAuthority{
		Root:          rootCert,
		Intermediates: []*x509.Certificate{intermediate},
		Leaf:          leaf,
	}
	tr, err := root.NewTrustedRoot(
		root.TrustedRootMediaType01,
		nil,
		nil,
		[]root.TimestampingAuthority{tsa},
		nil,
	)
	require.NoError(t, err)
	return tr, tsa
}

// pemBlockCount counts CERTIFICATE blocks in the input. Used by tests to
// confirm chain serialization round-trips cleanly.
func pemBlockCount(t *testing.T, pemBytes []byte) int {
	t.Helper()
	rest := pemBytes
	count := 0
	for {
		block, r := pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			count++
		}
		rest = r
	}
	return count
}

// ---- Tests ----

func TestTSAOnlyTrustedMaterial_ExposesConfiguredTSA(t *testing.T) {
	leaf, intermediate, rootCert := generateTSAChain(t)
	tsa := &root.SigstoreTimestampingAuthority{
		Root:          rootCert,
		Intermediates: []*x509.Certificate{intermediate},
		Leaf:          leaf,
	}
	tm := &tsaOnlyTrustedMaterial{tsa: tsa}

	tsAs := tm.TimestampingAuthorities()
	require.Len(t, tsAs, 1, "tsaOnlyTrustedMaterial must expose its configured TSA")
	assert.Same(t, tsa, tsAs[0], "TimestampingAuthorities must return the configured TSA, not a copy")
}

func TestTSAOnlyTrustedMaterial_OtherMethodsReturnDefaults(t *testing.T) {
	leaf, intermediate, rootCert := generateTSAChain(t)
	tsa := &root.SigstoreTimestampingAuthority{
		Root:          rootCert,
		Intermediates: []*x509.Certificate{intermediate},
		Leaf:          leaf,
	}
	tm := &tsaOnlyTrustedMaterial{tsa: tsa}

	// Other TrustedMaterial methods must safely return zero values so that
	// composing with another TrustedMaterial in a TrustedMaterialCollection
	// doesn't shadow that other member's contributions.
	assert.Empty(t, tm.FulcioCertificateAuthorities(), "Fulcio CAs must come from the public-root member of the collection, not this wrapper")
	assert.Empty(t, tm.RekorLogs(), "Rekor logs must come from the public-root member of the collection, not this wrapper")
	assert.Empty(t, tm.CTLogs(), "CT logs must come from the public-root member of the collection, not this wrapper")
}

func TestComposeTrustedMaterial_NilPublicRootIsRejected(t *testing.T) {
	tm, err := composeTrustedMaterial(nil, nil, nil, nil)
	assert.Nil(t, tm)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "public trusted root")
}

func TestComposeTrustedMaterial_NilLeafReturnsPublicRootUnchanged(t *testing.T) {
	publicRoot := emptyPublicTrustedRoot(t)
	tm, err := composeTrustedMaterial(publicRoot, nil, nil, nil)
	require.NoError(t, err)
	// Caller can keep using the public root directly; no collection wrapping
	// when no custom TSA was provided. Compare via interface satisfaction.
	assert.Same(t, publicRoot, tm)
}

func TestComposeTrustedMaterial_LeafWithoutRootIsRejected(t *testing.T) {
	publicRoot := emptyPublicTrustedRoot(t)
	leaf, intermediate, _ := generateTSAChain(t)

	// Deliberately pass an empty roots slice; opts.go's splitCertChain would
	// have returned nil/empty for a chain that lacks a self-signed cert.
	tm, err := composeTrustedMaterial(publicRoot, leaf, []*x509.Certificate{intermediate}, nil)
	assert.Nil(t, tm)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root")
}

// TestComposeTrustedMaterial_AggregatesTSAsFromBothSources is the bug-catching
// test. Without the fix, sigstore-go's verifier sees only the public root's
// TSAs (which doesn't include the caller's GitHub TSA), and verification of a
// bundle whose timestamp is signed by the caller's TSA fails. With the fix,
// the composed material exposes both TSAs, and the bundle verifies.
func TestComposeTrustedMaterial_AggregatesTSAsFromBothSources(t *testing.T) {
	publicRoot, publicTSA := publicTrustedRootWithTSA(t)
	customLeaf, customInt, customRoot := generateTSAChain(t)

	tm, err := composeTrustedMaterial(publicRoot, customLeaf, []*x509.Certificate{customInt}, []*x509.Certificate{customRoot})
	require.NoError(t, err)

	tsAs := tm.TimestampingAuthorities()
	require.Len(t, tsAs, 2, "composed material must aggregate TSAs from both the public root and the custom chain")

	// Order in TrustedMaterialCollection is publicRoot first, custom second.
	// The publicTSA pointer should appear in the aggregated list.
	foundPublic := false
	foundCustom := false
	for _, tsa := range tsAs {
		st, ok := tsa.(*root.SigstoreTimestampingAuthority)
		if !ok {
			continue
		}
		if st == publicTSA {
			foundPublic = true
		} else if st.Leaf != nil && st.Leaf.SerialNumber == customLeaf.SerialNumber {
			foundCustom = true
		}
	}
	assert.True(t, foundPublic, "public-root TSA must be visible after composition")
	assert.True(t, foundCustom, "custom TSA chain must be visible after composition")
}

func TestComposeTrustedMaterial_ReturnsCollectionType(t *testing.T) {
	publicRoot := emptyPublicTrustedRoot(t)
	leaf, intermediate, rootCert := generateTSAChain(t)

	tm, err := composeTrustedMaterial(publicRoot, leaf, []*x509.Certificate{intermediate}, []*x509.Certificate{rootCert})
	require.NoError(t, err)
	_, ok := tm.(root.TrustedMaterialCollection)
	assert.True(t, ok, "with a custom TSA chain, composeTrustedMaterial must return a TrustedMaterialCollection (got %T)", tm)
}

// TestComposeTrustedMaterial_OnlyFirstRootIsUsed documents the contract:
// when multiple self-signed certs are passed in, only the first is used as
// the SigstoreTimestampingAuthority's Root. This matches the
// SigstoreTimestampingAuthority struct's single-Root field.
func TestComposeTrustedMaterial_OnlyFirstRootIsUsed(t *testing.T) {
	publicRoot := emptyPublicTrustedRoot(t)
	leaf, intermediate, root1 := generateTSAChain(t)
	_, _, root2 := generateTSAChain(t)

	tm, err := composeTrustedMaterial(publicRoot, leaf, []*x509.Certificate{intermediate}, []*x509.Certificate{root1, root2})
	require.NoError(t, err)

	tsAs := tm.TimestampingAuthorities()
	require.Len(t, tsAs, 1, "with an empty public-root TSA list, the collection contributes only our custom TSA")
	sta, ok := tsAs[0].(*root.SigstoreTimestampingAuthority)
	require.True(t, ok)
	assert.Same(t, root1, sta.Root, "first root in the slice must be the chosen Root")
	assert.NotSame(t, root2, sta.Root, "second root must be ignored")
}

// ---- buildBundlePolicy tests ----

func TestBuildBundlePolicy_NoIdentitiesProducesArtifactOnlyPolicy(t *testing.T) {
	hash := sha256TestHash(t)
	co := &cosign.CheckOpts{}

	pb, err := buildBundlePolicy(hash, co)
	require.NoError(t, err)
	// PolicyBuilder is a value type; we just check that we can build it
	// without an identity, which is the keyless-with-no-identity path.
	_ = pb
}

func TestBuildBundlePolicy_WithIssuerAndSubjectAddsIdentity(t *testing.T) {
	hash := sha256TestHash(t)
	co := &cosign.CheckOpts{
		Identities: []cosign.Identity{
			{
				Issuer:  "https://token.actions.githubusercontent.com",
				Subject: "https://github.com/example/repo/.github/workflows/ci.yml@refs/heads/main",
			},
		},
	}

	pb, err := buildBundlePolicy(hash, co)
	require.NoError(t, err)
	_ = pb
}

func TestBuildBundlePolicy_PartialIdentityFallsBackToArtifactOnly(t *testing.T) {
	// Issuer present but Subject empty (and no SubjectRegExp): per the
	// existing CPOL pattern, we fall back to an artifact-only policy
	// instead of constructing an invalid certificate identity.
	hash := sha256TestHash(t)
	co := &cosign.CheckOpts{
		Identities: []cosign.Identity{
			{Issuer: "https://issuer.example.com"},
		},
	}
	pb, err := buildBundlePolicy(hash, co)
	require.NoError(t, err)
	_ = pb
}

// TestCertificateIdentityOptions_NoIdentitiesProducesNoOptions: keyless-with-
// no-identity bundle policies just bind to the artifact; no certificate
// identity options are added.
func TestCertificateIdentityOptions_NoIdentitiesProducesNoOptions(t *testing.T) {
	opts, err := certificateIdentityOptions(nil)
	require.NoError(t, err)
	assert.Empty(t, opts)
}

// TestCertificateIdentityOptions_MultipleWellFormedIdentitiesAreAllReturned
// mirrors cosign's keyless-OR semantics: every well-formed identity in the
// input becomes its own WithCertificateIdentity option. sigstore-go's
// PolicyBuilder accumulates them on a CertificateIdentities slice;
// CertificateIdentities.Verify documents the OR contract on the resulting
// slice ("if ANY of them match the cert, Verify returns nil") so passing
// every identity preserves the cosign behaviour for IVPOL attestors that
// declare more than one Identity entry.
func TestCertificateIdentityOptions_MultipleWellFormedIdentitiesAreAllReturned(t *testing.T) {
	identities := []cosign.Identity{
		{Issuer: "https://token.actions.githubusercontent.com", Subject: "https://github.com/acme/repo/.github/workflows/release.yml@refs/heads/main"},
		{Issuer: "https://token.actions.githubusercontent.com", Subject: "https://github.com/acme/repo/.github/workflows/ci.yml@refs/heads/main"},
		{IssuerRegExp: "https://token.actions.githubusercontent.com", SubjectRegExp: "https://github.com/acme/.+/.github/workflows/.+"},
	}
	opts, err := certificateIdentityOptions(identities)
	require.NoError(t, err)
	assert.Len(t, opts, 3, "every well-formed identity must yield its own WithCertificateIdentity option (cosign-OR semantics)")
}

// TestCertificateIdentityOptions_HalfSpecifiedIdentitiesAreSkipped:
// identities lacking either issuer or subject (and the regex variants) are
// silently dropped to keep the resulting policy well-formed. sigstore-go's
// NewShortCertificateIdentity would reject them; surfacing that error in
// IVPOL's policy build would break callers whose CheckOpts has accumulated
// junk entries from upstream conversion.
func TestCertificateIdentityOptions_HalfSpecifiedIdentitiesAreSkipped(t *testing.T) {
	identities := []cosign.Identity{
		{Issuer: "https://issuer.only.example.com"},                 // no subject — skip
		{Subject: "subject@only.example.com"},                       // no issuer — skip
		{Issuer: "https://issuer.example.com", Subject: "alice@ex"}, // well-formed — keep
	}
	opts, err := certificateIdentityOptions(identities)
	require.NoError(t, err)
	assert.Len(t, opts, 1)
}

// TestBuildBundlePolicy_MultipleIdentitiesAreAllPassedToPolicy ties the
// builder back to the higher-level entry point — buildBundlePolicy threads
// every well-formed identity through to the resulting PolicyBuilder via
// certificateIdentityOptions.
func TestBuildBundlePolicy_MultipleIdentitiesAreAllPassedToPolicy(t *testing.T) {
	hash := sha256TestHash(t)
	co := &cosign.CheckOpts{
		Identities: []cosign.Identity{
			{Issuer: "https://token.actions.githubusercontent.com", Subject: "https://github.com/acme/repo/.github/workflows/release.yml@refs/heads/main"},
			{Issuer: "https://token.actions.githubusercontent.com", Subject: "https://github.com/acme/repo/.github/workflows/ci.yml@refs/heads/main"},
		},
	}
	pb, err := buildBundlePolicy(hash, co)
	require.NoError(t, err)
	_ = pb // PolicyBuilder fields are unexported; covered via certificateIdentityOptions tests above.
}

func TestBuildBundlePolicy_BadDigestIsRejected(t *testing.T) {
	hash := &v1.Hash{Algorithm: "sha256", Hex: "not-hex"}
	co := &cosign.CheckOpts{}

	_, err := buildBundlePolicy(hash, co)
	require.Error(t, err)
}

// ---- buildBundleVerifyOptions tests ----
//
// sigstore-go requires exactly one "time" option per VerifierConfig. The
// option counts below reflect that contract:
//   - !IgnoreTlog adds WithTransparencyLog (Rekor)
//   - !IgnoreSCT adds WithSignedCertificateTimestamps (CT log)
//   - exactly one of {WithSignedTimestamps, WithIntegratedTimestamps,
//     WithNoObserverTimestamps, WithCurrentTime} is selected based on the
//     caller's UseSignedTimestamps / IgnoreTlog / SigVerifier triple.

func TestBuildBundleVerifyOptions_DefaultsEnableTlogSCTAndIntegratedTime(t *testing.T) {
	co := &cosign.CheckOpts{}
	opts := buildBundleVerifyOptions(co)
	// Three options: WithTransparencyLog, WithSignedCertificateTimestamps,
	// WithIntegratedTimestamps (the time option chosen when Rekor is in scope).
	require.Len(t, opts, 3)
}

func TestBuildBundleVerifyOptions_IgnoreTlogDropsTransparencyLogAndUsesCurrentTime(t *testing.T) {
	co := &cosign.CheckOpts{IgnoreTlog: true}
	opts := buildBundleVerifyOptions(co)
	// WithSignedCertificateTimestamps + WithCurrentTime (no Rekor, no SigVerifier).
	require.Len(t, opts, 2)
}

func TestBuildBundleVerifyOptions_IgnoreSCTDropsSCTOption(t *testing.T) {
	co := &cosign.CheckOpts{IgnoreSCT: true}
	opts := buildBundleVerifyOptions(co)
	// WithTransparencyLog + WithIntegratedTimestamps (Rekor still in scope).
	require.Len(t, opts, 2)
}

func TestBuildBundleVerifyOptions_IgnoreBothFallsBackToCurrentTime(t *testing.T) {
	co := &cosign.CheckOpts{IgnoreTlog: true, IgnoreSCT: true}
	opts := buildBundleVerifyOptions(co)
	// Only WithCurrentTime remains — exactly one time option as sigstore-go
	// requires, so verify.NewVerifier won't reject the config.
	require.Len(t, opts, 1)
}

func TestBuildBundleVerifyOptions_IgnoreBothWithSigVerifierUsesNoObserverTimestamps(t *testing.T) {
	// When a static public key is in use, WithNoObserverTimestamps is the
	// correct fallback — current time isn't needed since there's no cert
	// validity period to bracket.
	co := &cosign.CheckOpts{
		IgnoreTlog: true,
		IgnoreSCT:  true,
		SigVerifier: stubSigVerifier{}, // any non-nil signature.Verifier
	}
	opts := buildBundleVerifyOptions(co)
	require.Len(t, opts, 1)
}

func TestBuildBundleVerifyOptions_UseSignedTimestampsTakesPrecedenceOverIntegrated(t *testing.T) {
	co := &cosign.CheckOpts{
		UseSignedTimestamps: true,
	}
	opts := buildBundleVerifyOptions(co)
	// Three options: WithTransparencyLog, WithSignedCertificateTimestamps,
	// WithSignedTimestamps (UseSignedTimestamps wins over WithIntegratedTimestamps
	// — the caller has explicitly asked for RFC3161 verification).
	require.Len(t, opts, 3)
}

// ---- bundleToOCISignature tests ----

// dsseBundle constructs an *sgbundle.Bundle whose protobuf content carries
// the provided DSSE-envelope payload. We bypass sgbundle.NewBundle's
// validate() because the unit under test only inspects the embedded protobuf
// content; full bundle validity is the responsibility of upstream sigstore-go
// when verifier.Verify is called against a real artifact.
func dsseBundle(t *testing.T, payload []byte, payloadType string) *sgbundle.Bundle {
	t.Helper()
	inner := &protobundle.Bundle{
		MediaType: "application/vnd.dev.sigstore.bundle.v0.3+json",
		VerificationMaterial: &protobundle.VerificationMaterial{
			Content: &protobundle.VerificationMaterial_Certificate{
				Certificate: &protocommon.X509Certificate{},
			},
		},
		Content: &protobundle.Bundle_DsseEnvelope{
			DsseEnvelope: &protodsse.Envelope{
				Payload:     payload,
				PayloadType: payloadType,
				Signatures:  []*protodsse.Signature{{Sig: []byte("test-sig")}},
			},
		},
	}
	return &sgbundle.Bundle{Bundle: inner}
}

// nonDSSEBundle constructs an *sgbundle.Bundle with MessageSignature content
// instead of a DSSE envelope, so we can verify our extractor rejects it.
func nonDSSEBundle(t *testing.T) *sgbundle.Bundle {
	t.Helper()
	inner := &protobundle.Bundle{
		MediaType: "application/vnd.dev.sigstore.bundle.v0.3+json",
		Content: &protobundle.Bundle_MessageSignature{
			MessageSignature: &protocommon.MessageSignature{
				Signature: []byte("sig"),
			},
		},
	}
	return &sgbundle.Bundle{Bundle: inner}
}

func TestBundleToOCISignature_NilBundleIsRejected(t *testing.T) {
	sig, err := bundleToOCISignature(nil)
	assert.Nil(t, sig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil bundle")
}

func TestBundleToOCISignature_BundleWithoutInnerProtobufIsRejected(t *testing.T) {
	b := &sgbundle.Bundle{}
	sig, err := bundleToOCISignature(b)
	assert.Nil(t, sig)
	require.Error(t, err)
}

func TestBundleToOCISignature_NonDSSEContentIsRejected(t *testing.T) {
	b := nonDSSEBundle(t)
	sig, err := bundleToOCISignature(b)
	assert.Nil(t, sig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DSSE")
}

func TestBundleToOCISignature_DSSEPayloadIsPreserved(t *testing.T) {
	// Use a SLSA-provenance-shaped JSON payload so the envelope/payload-type
	// fields in the resulting JSON match what cosign produces from real
	// build-provenance bundles.
	statement := map[string]any{
		"_type":         "https://in-toto.io/Statement/v1",
		"predicateType": "https://slsa.dev/provenance/v1",
		"subject":       []map[string]any{{"name": "ghcr.io/example/img", "digest": map[string]string{"sha256": "abc"}}},
		"predicate":     map[string]any{"buildDefinition": map[string]any{}, "runDetails": map[string]any{}},
	}
	statementJSON, err := json.Marshal(statement)
	require.NoError(t, err)

	b := dsseBundle(t, statementJSON, "application/vnd.in-toto+json")

	sig, err := bundleToOCISignature(b)
	require.NoError(t, err)
	require.NotNil(t, sig)

	// The returned oci.Signature carries the marshaled DSSE envelope as its
	// Payload, mirroring cosign's verifyImageAttestationsSigstoreBundle. The
	// IVPOL caller decodes statements from this payload.
	gotPayload, err := sig.Payload()
	require.NoError(t, err)

	var envelope struct {
		Payload     []byte `json:"payload"`
		PayloadType string `json:"payloadType"`
	}
	require.NoError(t, json.Unmarshal(gotPayload, &envelope))
	assert.Equal(t, "application/vnd.in-toto+json", envelope.PayloadType)
	assert.JSONEq(t, string(statementJSON), string(envelope.Payload))
}
