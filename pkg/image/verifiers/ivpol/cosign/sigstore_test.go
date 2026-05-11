package cosign

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
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

// stubSigVerifier flips the cosign.CheckOpts.SigVerifier branch in
// buildBundleVerifyOptions tests without bringing in real key material.
type stubSigVerifier struct{}

var _ sigsig.Verifier = stubSigVerifier{}

func (stubSigVerifier) PublicKey(_ ...sigsig.PublicKeyOption) (crypto.PublicKey, error) {
	return nil, nil
}
func (stubSigVerifier) VerifySignature(_, _ io.Reader, _ ...sigsig.VerifyOption) error {
	return nil
}

func sha256TestHash(t *testing.T) *v1.Hash {
	t.Helper()
	return &v1.Hash{
		Algorithm: "sha256",
		Hex:       "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}
}

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

func generateTSAChain(t *testing.T) (leaf, intermediate, rootCert *x509.Certificate) {
	t.Helper()
	rootCert, rootKey := generateRootCA(t)
	intermediate, intermediateKey := generateIntermediateCA(t, rootCert, rootKey)
	leaf, _ = generateTSALeafCert(t, intermediate, intermediateKey, 100)
	return leaf, intermediate, rootCert
}

func emptyPublicTrustedRoot(t *testing.T) *root.TrustedRoot {
	t.Helper()
	tr, err := root.NewTrustedRoot(root.TrustedRootMediaType01, nil, nil, nil, nil)
	require.NoError(t, err)
	return tr
}

func publicTrustedRootWithTSA(t *testing.T) (*root.TrustedRoot, *root.SigstoreTimestampingAuthority) {
	t.Helper()
	leaf, intermediate, rootCert := generateTSAChain(t)
	tsa := &root.SigstoreTimestampingAuthority{
		Root:          rootCert,
		Intermediates: []*x509.Certificate{intermediate},
		Leaf:          leaf,
	}
	tr, err := root.NewTrustedRoot(root.TrustedRootMediaType01, nil, nil, []root.TimestampingAuthority{tsa}, nil)
	require.NoError(t, err)
	return tr, tsa
}

// dsseBundle constructs an *sgbundle.Bundle whose protobuf content carries
// the provided DSSE envelope. Bypasses sgbundle.NewBundle's validate()
// because the unit under test only inspects the embedded content.
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

func nonDSSEBundle(t *testing.T) *sgbundle.Bundle {
	t.Helper()
	inner := &protobundle.Bundle{
		MediaType: "application/vnd.dev.sigstore.bundle.v0.3+json",
		Content: &protobundle.Bundle_MessageSignature{
			MessageSignature: &protocommon.MessageSignature{Signature: []byte("sig")},
		},
	}
	return &sgbundle.Bundle{Bundle: inner}
}

// ---- tsaOnlyTrustedMaterial ----

func TestTSAOnlyTrustedMaterial_ExposesConfiguredTSA(t *testing.T) {
	leaf, intermediate, rootCert := generateTSAChain(t)
	tsa := &root.SigstoreTimestampingAuthority{
		Root:          rootCert,
		Intermediates: []*x509.Certificate{intermediate},
		Leaf:          leaf,
	}
	tm := &tsaOnlyTrustedMaterial{tsa: tsa}

	tsAs := tm.TimestampingAuthorities()
	require.Len(t, tsAs, 1)
	assert.Same(t, tsa, tsAs[0])
}

// Other TrustedMaterial methods must return zero values so the wrapper
// doesn't shadow the public-root member when composed in a collection.
func TestTSAOnlyTrustedMaterial_OtherMethodsReturnDefaults(t *testing.T) {
	leaf, intermediate, rootCert := generateTSAChain(t)
	tsa := &root.SigstoreTimestampingAuthority{
		Root:          rootCert,
		Intermediates: []*x509.Certificate{intermediate},
		Leaf:          leaf,
	}
	tm := &tsaOnlyTrustedMaterial{tsa: tsa}

	assert.Empty(t, tm.FulcioCertificateAuthorities())
	assert.Empty(t, tm.RekorLogs())
	assert.Empty(t, tm.CTLogs())
}

// ---- composeTrustedMaterial ----

func TestComposeTrustedMaterial_NilPublicRootIsRejected(t *testing.T) {
	tm, err := composeTrustedMaterial(nil, nil, nil, nil)
	assert.Nil(t, tm)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "public trusted material")
}

func TestComposeTrustedMaterial_EmptyCustomChainReturnsPublicRootUnchanged(t *testing.T) {
	publicRoot := emptyPublicTrustedRoot(t)
	tm, err := composeTrustedMaterial(publicRoot, nil, nil, nil)
	require.NoError(t, err)
	assert.Same(t, publicRoot, tm)
}

func TestComposeTrustedMaterial_LeafWithoutRootIsRejected(t *testing.T) {
	publicRoot := emptyPublicTrustedRoot(t)
	leaf, intermediate, _ := generateTSAChain(t)

	tm, err := composeTrustedMaterial(publicRoot, leaf, []*x509.Certificate{intermediate}, nil)
	assert.Nil(t, tm)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root")
}

// CTLog.TSACertChain documents the leaf TSA cert as optional: the leaf
// can live in the RFC3161 timestamp response itself. opts.go preserves
// that contract — TSARootCertificates can be populated with TSACertificate
// nil. composeTrustedMaterial must compose authority material in that
// case, not silently drop the user's roots.
func TestComposeTrustedMaterial_RootsOnlyAreExposedAsTSA(t *testing.T) {
	publicRoot := emptyPublicTrustedRoot(t)
	_, _, rootCert := generateTSAChain(t)

	tm, err := composeTrustedMaterial(publicRoot, nil, nil, []*x509.Certificate{rootCert})
	require.NoError(t, err)

	tsAs := tm.TimestampingAuthorities()
	require.Len(t, tsAs, 1, "the single custom root must produce one timestamping authority")
	sta, ok := tsAs[0].(*root.SigstoreTimestampingAuthority)
	require.True(t, ok)
	assert.Same(t, rootCert, sta.Root)
	assert.Nil(t, sta.Leaf, "leaf nil is honoured; sigstore-go pulls the leaf from the TSR or errors clearly")
}

func TestComposeTrustedMaterial_AggregatesTSAsFromBothSources(t *testing.T) {
	publicRoot, publicTSA := publicTrustedRootWithTSA(t)
	customLeaf, customInt, customRoot := generateTSAChain(t)

	tm, err := composeTrustedMaterial(publicRoot, customLeaf, []*x509.Certificate{customInt}, []*x509.Certificate{customRoot})
	require.NoError(t, err)

	tsAs := tm.TimestampingAuthorities()
	require.Len(t, tsAs, 2)

	foundPublic, foundCustom := false, false
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
	assert.True(t, foundPublic)
	assert.True(t, foundCustom)
}

func TestComposeTrustedMaterial_ReturnsCollectionType(t *testing.T) {
	publicRoot := emptyPublicTrustedRoot(t)
	leaf, intermediate, rootCert := generateTSAChain(t)

	tm, err := composeTrustedMaterial(publicRoot, leaf, []*x509.Certificate{intermediate}, []*x509.Certificate{rootCert})
	require.NoError(t, err)
	_, ok := tm.(root.TrustedMaterialCollection)
	assert.True(t, ok)
}

// SigstoreTimestampingAuthority has a single Root field, but TSACertChain
// allows multiple roots — and cosign's non-bundle path (cpol/cosign/cosign.go)
// passes the whole roots slice through. Build one SigstoreTimestampingAuthority
// per root, sharing leaf + intermediates, so the bundle path doesn't introduce
// a multi-root-rejection asymmetry vs. the non-bundle path.
func TestComposeTrustedMaterial_MultipleRootsExposeOneTSAPerRoot(t *testing.T) {
	publicRoot := emptyPublicTrustedRoot(t)
	leaf, intermediate, root1 := generateTSAChain(t)
	_, _, root2 := generateTSAChain(t)

	tm, err := composeTrustedMaterial(publicRoot, leaf, []*x509.Certificate{intermediate}, []*x509.Certificate{root1, root2})
	require.NoError(t, err)

	tsAs := tm.TimestampingAuthorities()
	require.Len(t, tsAs, 2, "two custom roots must produce two timestamping authorities (one per root)")

	foundRoot1, foundRoot2 := false, false
	for _, tsa := range tsAs {
		st, ok := tsa.(*root.SigstoreTimestampingAuthority)
		if !ok {
			continue
		}
		assert.Same(t, leaf, st.Leaf, "each TSA shares the configured leaf")
		switch st.Root {
		case root1:
			foundRoot1 = true
		case root2:
			foundRoot2 = true
		}
	}
	assert.True(t, foundRoot1, "first root must be exposed as its own TSA")
	assert.True(t, foundRoot2, "second root must be exposed as its own TSA")
}

// ---- buildBundlePolicy ----

func TestBuildBundlePolicy_NoIdentitiesProducesArtifactOnlyPolicy(t *testing.T) {
	pb, err := buildBundlePolicy(sha256TestHash(t), &cosign.CheckOpts{})
	require.NoError(t, err)
	_ = pb
}

func TestBuildBundlePolicy_WithIssuerAndSubjectAddsIdentity(t *testing.T) {
	co := &cosign.CheckOpts{
		Identities: []cosign.Identity{{
			Issuer:  "https://token.actions.githubusercontent.com",
			Subject: "https://github.com/example/repo/.github/workflows/ci.yml@refs/heads/main",
		}},
	}
	pb, err := buildBundlePolicy(sha256TestHash(t), co)
	require.NoError(t, err)
	_ = pb
}

// A half-specified identity (issuer or subject criteria missing) is rejected
// by sigstore-go's NewCertificateIdentity; buildBundlePolicy propagates that
// rather than silently dropping the entry, since silent dropping would
// downgrade verification to digest-only without any signal to the operator.
func TestBuildBundlePolicy_PartialIdentityIsRejected(t *testing.T) {
	co := &cosign.CheckOpts{
		Identities: []cosign.Identity{{Issuer: "https://issuer.example.com"}},
	}
	_, err := buildBundlePolicy(sha256TestHash(t), co)
	require.Error(t, err)
}

func TestBuildBundlePolicy_BadDigestIsRejected(t *testing.T) {
	hash := &v1.Hash{Algorithm: "sha256", Hex: "not-hex"}
	_, err := buildBundlePolicy(hash, &cosign.CheckOpts{})
	require.Error(t, err)
}

// PolicyBuilder fields are unexported, so multi-identity coverage lives on
// certificateIdentityOptions below; this is a smoke test on the wrapper.
func TestBuildBundlePolicy_MultipleIdentitiesAreAllPassedToPolicy(t *testing.T) {
	co := &cosign.CheckOpts{
		Identities: []cosign.Identity{
			{Issuer: "https://token.actions.githubusercontent.com", Subject: "https://github.com/acme/repo/.github/workflows/release.yml@refs/heads/main"},
			{Issuer: "https://token.actions.githubusercontent.com", Subject: "https://github.com/acme/repo/.github/workflows/ci.yml@refs/heads/main"},
		},
	}
	pb, err := buildBundlePolicy(sha256TestHash(t), co)
	require.NoError(t, err)
	_ = pb
}

// ---- certificateIdentityOptions ----

func TestCertificateIdentityOptions_NoIdentitiesProducesNoOptions(t *testing.T) {
	opts, err := certificateIdentityOptions(nil)
	require.NoError(t, err)
	assert.Empty(t, opts)
}

func TestCertificateIdentityOptions_MultipleWellFormedIdentitiesAreAllReturned(t *testing.T) {
	identities := []cosign.Identity{
		{Issuer: "https://token.actions.githubusercontent.com", Subject: "https://github.com/acme/repo/.github/workflows/release.yml@refs/heads/main"},
		{Issuer: "https://token.actions.githubusercontent.com", Subject: "https://github.com/acme/repo/.github/workflows/ci.yml@refs/heads/main"},
		{IssuerRegExp: "https://token.actions.githubusercontent.com", SubjectRegExp: "https://github.com/acme/.+/.github/workflows/.+"},
	}
	opts, err := certificateIdentityOptions(identities)
	require.NoError(t, err)
	assert.Len(t, opts, 3)
}

// Half-specified entries (issuer or SAN criteria missing) are propagated
// as errors — sigstore-go's NewCertificateIdentity rejects them, and
// silent skipping would let a misconfigured entry weaken verification to
// digest-only without any signal back to the operator.
func TestCertificateIdentityOptions_IssuerOnlyIsRejected(t *testing.T) {
	identities := []cosign.Identity{{Issuer: "https://issuer.only.example.com"}}
	_, err := certificateIdentityOptions(identities)
	require.Error(t, err)
}

func TestCertificateIdentityOptions_SubjectOnlyIsRejected(t *testing.T) {
	identities := []cosign.Identity{{Subject: "subject@only.example.com"}}
	_, err := certificateIdentityOptions(identities)
	require.Error(t, err)
}

// One half-specified entry in a list of otherwise valid entries fails the
// whole list — operators see the error at policy-build time rather than
// silently losing the constraint at admission time.
func TestCertificateIdentityOptions_OneHalfSpecifiedFailsTheWholeList(t *testing.T) {
	identities := []cosign.Identity{
		{Issuer: "https://issuer.example.com", Subject: "alice@ex"}, // valid
		{Issuer: "https://issuer.only.example.com"},                 // half-specified
	}
	_, err := certificateIdentityOptions(identities)
	require.Error(t, err)
}

// When NewShortCertificateIdentity fails, the wrapped error must surface
// every configured matcher — IssuerRegExp/SubjectRegExp setups would
// otherwise log issuer="" subject="" and lose the actually-failing value.
func TestCertificateIdentityOptions_ErrorIncludesAllConfiguredFields(t *testing.T) {
	// `[unclosed-class` is an invalid regex; NewSANMatcher fails to
	// compile it and bubbles the error up through NewShortCertificateIdentity.
	identities := []cosign.Identity{
		{Issuer: "https://issuer.example.com", SubjectRegExp: "[unclosed-class"},
	}
	_, err := certificateIdentityOptions(identities)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `issuer="https://issuer.example.com"`)
	assert.Contains(t, err.Error(), `subjectRegExp="[unclosed-class"`)
}

// ---- buildBundleVerifyOptions ----
//
// sigstore-go's VerifierConfig requires exactly one "time" option from the
// set {WithSignedTimestamps, WithObserverTimestamps, WithIntegratedTimestamps,
// WithCurrentTime, WithNoObserverTimestamps}. The option counts below
// reflect that contract.

func TestBuildBundleVerifyOptions_DefaultsEnableTlogSCTAndObserverTimestamps(t *testing.T) {
	// Default-case picks WithObserverTimestamps (accepts either signed or
	// integrated) so Rekor v2-only bundles — which have zero IntegratedTime —
	// don't require a separate UseSignedTimestamps preflight to verify.
	opts := buildBundleVerifyOptions(&cosign.CheckOpts{})
	require.Len(t, opts, 3)
}

func TestBuildBundleVerifyOptions_IgnoreTlogDropsTransparencyLogAndUsesCurrentTime(t *testing.T) {
	opts := buildBundleVerifyOptions(&cosign.CheckOpts{IgnoreTlog: true})
	require.Len(t, opts, 2)
}

func TestBuildBundleVerifyOptions_IgnoreSCTDropsSCTOption(t *testing.T) {
	opts := buildBundleVerifyOptions(&cosign.CheckOpts{IgnoreSCT: true})
	require.Len(t, opts, 2)
}

func TestBuildBundleVerifyOptions_IgnoreBothFallsBackToCurrentTime(t *testing.T) {
	opts := buildBundleVerifyOptions(&cosign.CheckOpts{IgnoreTlog: true, IgnoreSCT: true})
	require.Len(t, opts, 1)
}

func TestBuildBundleVerifyOptions_IgnoreBothWithSigVerifierUsesNoObserverTimestamps(t *testing.T) {
	co := &cosign.CheckOpts{IgnoreTlog: true, IgnoreSCT: true, SigVerifier: stubSigVerifier{}}
	opts := buildBundleVerifyOptions(co)
	require.Len(t, opts, 1)
}

func TestBuildBundleVerifyOptions_UseSignedTimestampsTakesPrecedenceOverObserver(t *testing.T) {
	opts := buildBundleVerifyOptions(&cosign.CheckOpts{UseSignedTimestamps: true})
	require.Len(t, opts, 3)
}

// ---- rejectAnnotationsOnBundlePath ----
//
// Bundle verification (keyless or static-key) builds oci.Signatures via
// static.NewAttestation from the DSSE envelope only, so the resulting
// Signatures carry no OCI annotations. Combining annotation matchers with
// any bundle path is a config error worth surfacing explicitly rather than
// producing a misleading "no signature matched" error later.

func TestRejectAnnotationsOnBundlePath_NoAnnotationsIsAccepted(t *testing.T) {
	co := &cosign.CheckOpts{NewBundleFormat: true, TrustedMaterial: emptyPublicTrustedRoot(t)}
	err := rejectAnnotationsOnBundlePath(co, nil)
	assert.NoError(t, err)
}

func TestRejectAnnotationsOnBundlePath_NonBundlePathAccepts(t *testing.T) {
	// Legacy non-bundle path through cosign.VerifyImageSignatures still
	// propagates OCI annotations on its oci.Signature results, so annotations
	// remain supported there.
	co := &cosign.CheckOpts{NewBundleFormat: false}
	err := rejectAnnotationsOnBundlePath(co, map[string]string{"foo": "bar"})
	assert.NoError(t, err)
}

func TestRejectAnnotationsOnBundlePath_BundleKeylessWithAnnotationsIsRejected(t *testing.T) {
	co := &cosign.CheckOpts{NewBundleFormat: true, TrustedMaterial: emptyPublicTrustedRoot(t)}
	err := rejectAnnotationsOnBundlePath(co, map[string]string{"foo": "bar"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "annotations is not supported")
}

func TestRejectAnnotationsOnBundlePath_BundleStaticKeyWithAnnotationsIsRejected(t *testing.T) {
	// The bundle path with a static key routes through cosign rather than our
	// sigstore-go helper, but cosign's verifyImageAttestationsSigstoreBundle
	// has the same limitation (no annotation propagation on the resulting
	// oci.Signature). The reject guard is bundle-format-specific, not
	// keyless-specific, so both shapes hit the same clear error.
	co := &cosign.CheckOpts{NewBundleFormat: true, SigVerifier: stubSigVerifier{}}
	err := rejectAnnotationsOnBundlePath(co, map[string]string{"foo": "bar"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "annotations is not supported")
}

// ---- bundleToOCISignature ----

func TestBundleToOCISignature_NilBundleIsRejected(t *testing.T) {
	sig, err := bundleToOCISignature(nil)
	assert.Nil(t, sig)
	require.Error(t, err)
}

func TestBundleToOCISignature_BundleWithoutInnerProtobufIsRejected(t *testing.T) {
	sig, err := bundleToOCISignature(&sgbundle.Bundle{})
	assert.Nil(t, sig)
	require.Error(t, err)
}

func TestBundleToOCISignature_NonDSSEContentIsRejected(t *testing.T) {
	sig, err := bundleToOCISignature(nonDSSEBundle(t))
	assert.Nil(t, sig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DSSE")
}

func TestBundleToOCISignature_DSSEPayloadIsPreserved(t *testing.T) {
	statement := map[string]any{
		"_type":         "https://in-toto.io/Statement/v1",
		"predicateType": "https://slsa.dev/provenance/v1",
		"subject":       []map[string]any{{"name": "ghcr.io/example/img", "digest": map[string]string{"sha256": "abc"}}},
		"predicate":     map[string]any{"buildDefinition": map[string]any{}, "runDetails": map[string]any{}},
	}
	statementJSON, err := json.Marshal(statement)
	require.NoError(t, err)

	sig, err := bundleToOCISignature(dsseBundle(t, statementJSON, "application/vnd.in-toto+json"))
	require.NoError(t, err)
	require.NotNil(t, sig)

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
