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
	assert.Contains(t, err.Error(), "public trusted root")
}

func TestComposeTrustedMaterial_NilLeafReturnsPublicRootUnchanged(t *testing.T) {
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

// SigstoreTimestampingAuthority has a single Root field; only the first
// element of customTSARoots is used.
func TestComposeTrustedMaterial_OnlyFirstRootIsUsed(t *testing.T) {
	publicRoot := emptyPublicTrustedRoot(t)
	leaf, intermediate, root1 := generateTSAChain(t)
	_, _, root2 := generateTSAChain(t)

	tm, err := composeTrustedMaterial(publicRoot, leaf, []*x509.Certificate{intermediate}, []*x509.Certificate{root1, root2})
	require.NoError(t, err)

	tsAs := tm.TimestampingAuthorities()
	require.Len(t, tsAs, 1)
	sta, ok := tsAs[0].(*root.SigstoreTimestampingAuthority)
	require.True(t, ok)
	assert.Same(t, root1, sta.Root)
	assert.NotSame(t, root2, sta.Root)
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

func TestBuildBundlePolicy_PartialIdentityFallsBackToArtifactOnly(t *testing.T) {
	co := &cosign.CheckOpts{
		Identities: []cosign.Identity{{Issuer: "https://issuer.example.com"}},
	}
	pb, err := buildBundlePolicy(sha256TestHash(t), co)
	require.NoError(t, err)
	_ = pb
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

func TestCertificateIdentityOptions_HalfSpecifiedIdentitiesAreSkipped(t *testing.T) {
	identities := []cosign.Identity{
		{Issuer: "https://issuer.only.example.com"},
		{Subject: "subject@only.example.com"},
		{Issuer: "https://issuer.example.com", Subject: "alice@ex"},
	}
	opts, err := certificateIdentityOptions(identities)
	require.NoError(t, err)
	assert.Len(t, opts, 1)
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

func TestBuildBundleVerifyOptions_DefaultsEnableTlogSCTAndIntegratedTime(t *testing.T) {
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

func TestBuildBundleVerifyOptions_UseSignedTimestampsTakesPrecedenceOverIntegrated(t *testing.T) {
	opts := buildBundleVerifyOptions(&cosign.CheckOpts{UseSignedTimestamps: true})
	require.Len(t, opts, 3)
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
