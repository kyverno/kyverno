package cosign

import (
	"bytes"
	"context"
	"crypto"
	"encoding/base64"
	"fmt"
	"os"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/sigstore/cosign/v3/pkg/oci/mutate"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	ocistatic "github.com/sigstore/cosign/v3/pkg/oci/static"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	protodsse "github.com/sigstore/protobuf-specs/gen/pb-go/dsse"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/testing/ca"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	offlineTestIssuer  = "https://issuer.example.com"
	offlineTestSubject = "builder@example.com"
)

// newPrivateSigstore returns a virtual Sigstore instance and its trusted root
// as JSON, shaped like GitHub's private instance: a Fulcio CA and a TSA, with
// no transparency or CT log.
func newPrivateSigstore(t *testing.T) (*ca.VirtualSigstore, string) {
	t.Helper()
	vs, err := ca.NewVirtualSigstore()
	require.NoError(t, err)
	return vs, composeTrustedRoot(t, vs, vs)
}

// composeTrustedRoot returns a trusted root as JSON with the Fulcio CAs of
// casFrom and the TSAs of tsasFrom (nil for none), and no transparency or CT
// log.
func composeTrustedRoot(t *testing.T, casFrom, tsasFrom *ca.VirtualSigstore) string {
	t.Helper()
	var cas []root.CertificateAuthority
	var tsas []root.TimestampingAuthority
	if casFrom != nil {
		cas = casFrom.FulcioCertificateAuthorities()
	}
	if tsasFrom != nil {
		tsas = tsasFrom.TimestampingAuthorities()
	}
	tr, err := root.NewTrustedRoot(root.TrustedRootMediaType01, cas, nil, tsas, nil)
	require.NoError(t, err)
	return trustedRootJSON(t, tr)
}

// signKeylessBundle builds a v0.3 bundle the way a private Sigstore instance
// does: a Fulcio leaf certificate and an RFC 3161 timestamp, no tlog entry.
func signKeylessBundle(t *testing.T, vs *ca.VirtualSigstore, subjectDigestHex string) []byte {
	t.Helper()
	statement := fmt.Sprintf(
		`{"_type":"https://in-toto.io/Statement/v1","predicateType":"https://slsa.dev/provenance/v1","subject":[{"name":"subject","digest":{"sha256":%q}}],"predicate":{}}`,
		subjectDigestHex,
	)
	entity, err := vs.Attest(offlineTestSubject, offlineTestIssuer, []byte(statement))
	require.NoError(t, err)

	vc, err := entity.VerificationContent()
	require.NoError(t, err)
	cert := vc.Certificate()
	require.NotNil(t, cert)
	sc, err := entity.SignatureContent()
	require.NoError(t, err)
	envelope := sc.EnvelopeContent().RawEnvelope()
	timestamps, err := entity.Timestamps()
	require.NoError(t, err)

	sigs := make([]*protodsse.Signature, 0, len(envelope.Signatures))
	for _, s := range envelope.Signatures {
		raw, err := base64.StdEncoding.DecodeString(s.Sig)
		require.NoError(t, err)
		sigs = append(sigs, &protodsse.Signature{Sig: raw, Keyid: s.KeyID})
	}
	payload, err := base64.StdEncoding.DecodeString(envelope.Payload)
	require.NoError(t, err)
	rfc3161 := make([]*protocommon.RFC3161SignedTimestamp, 0, len(timestamps))
	for _, ts := range timestamps {
		rfc3161 = append(rfc3161, &protocommon.RFC3161SignedTimestamp{SignedTimestamp: ts})
	}

	pb := &protobundle.Bundle{
		MediaType: testBundleMediaType,
		VerificationMaterial: &protobundle.VerificationMaterial{
			Content: &protobundle.VerificationMaterial_Certificate{
				Certificate: &protocommon.X509Certificate{RawBytes: cert.Raw},
			},
			TimestampVerificationData: &protobundle.TimestampVerificationData{Rfc3161Timestamps: rfc3161},
		},
		Content: &protobundle.Bundle_DsseEnvelope{DsseEnvelope: &protodsse.Envelope{
			Payload:     payload,
			PayloadType: envelope.PayloadType,
			Signatures:  sigs,
		}},
	}
	raw, err := protojson.Marshal(pb)
	require.NoError(t, err)
	return raw
}

func offlineKeylessAttestor(trustedRoot *v1beta1.StringOrExpression) *v1beta1.Attestor {
	return &v1beta1.Attestor{Name: "private-sigstore", Cosign: &v1beta1.Cosign{
		Keyless: &v1beta1.Keyless{Identities: []v1beta1.Identity{{
			Issuer:  offlineTestIssuer,
			Subject: offlineTestSubject,
		}}},
		CTLog:       &v1beta1.CTLog{InsecureIgnoreTlog: true, InsecureIgnoreSCT: true},
		TrustedRoot: trustedRoot,
	}}
}

// Test_KeylessInlineTrustedRoot_VerifiesWithoutTUF covers #17883 end to end: a
// keyless bundle from a private Sigstore instance verifies against the inline
// trusted root while every TUF access fails, as on a cluster without egress.
func Test_KeylessInlineTrustedRoot_VerifiesWithoutTUF(t *testing.T) {
	vs, trustedRoot := newPrivateSigstore(t)
	repo, _ := newBundleRegistry(t)
	subject := pushSubject(t, repo, "subject")
	pushReferrer(t, repo, subject, signKeylessBundle(t, vs, subject.Digest.Hex), testBundleMediaType, testBundleMediaType)
	img := fetchTestImage(t, repo.Tag("subject").String())
	attestation := &v1beta1.Attestation{Name: "slsa", InToto: &v1beta1.InToto{Type: "https://slsa.dev/provenance/v1"}}

	t.Run("signature and attestation verify against the inline root", func(t *testing.T) {
		tufCalls := stubTufUnreachable(t)
		v := NewVerifier(nil, logr.Discard())
		attestor := offlineKeylessAttestor(&v1beta1.StringOrExpression{Value: trustedRoot})
		require.NoError(t, v.VerifyImageSignature(context.TODO(), img, attestor))
		require.NoError(t, v.VerifyAttestationSignature(context.TODO(), img, attestation, attestor))
		require.Zero(t, tufCalls.Load())
	})

	other, otherRoot := newPrivateSigstore(t)
	refusals := []struct {
		name        string
		root        string
		errContains string
	}{
		{
			// The signer's TSA is trusted, so the refusal comes from the chain.
			name:        "another instance's Fulcio CA",
			root:        composeTrustedRoot(t, other, vs),
			errContains: "leaf certificate verification failed",
		},
		{
			name:        "another instance's root",
			root:        otherRoot,
			errContains: "threshold not met for verified signed timestamps",
		},
		{
			name:        "root with CAs but no TSAs, tlog ignored",
			root:        composeTrustedRoot(t, vs, nil),
			errContains: "threshold not met for verified signed timestamps",
		},
		{
			name:        "mediaType-only root",
			root:        `{"mediaType":"application/vnd.dev.sigstore.trustedroot+json;version=0.1"}`,
			errContains: "threshold not met for verified signed timestamps",
		},
	}
	for _, tt := range refusals {
		t.Run("refused: "+tt.name, func(t *testing.T) {
			tufCalls := stubTufUnreachable(t)
			v := NewVerifier(nil, logr.Discard())
			attestor := offlineKeylessAttestor(&v1beta1.StringOrExpression{Value: tt.root})

			err := v.VerifyImageSignature(context.TODO(), img, attestor)
			require.ErrorContains(t, err, tt.errContains)
			assert.NotContains(t, err.Error(), "TUF")

			err = v.VerifyAttestationSignature(context.TODO(), img, attestation, attestor)
			require.ErrorContains(t, err, tt.errContains)
			assert.NotContains(t, err.Error(), "TUF")
			assert.Zero(t, tufCalls.Load())
		})
	}

	t.Run("without an inline root TUF is still required", func(t *testing.T) {
		tufCalls := stubTufUnreachable(t)
		v := NewVerifier(nil, logr.Discard())
		err := v.VerifyImageSignature(context.TODO(), img, offlineKeylessAttestor(nil))
		require.ErrorContains(t, err, "failed to initialize TUF client")
		require.NotZero(t, tufCalls.Load())
	})
}

// signLegacyKeyless attaches a cosign v2 (non-bundle) keyless signature, a
// Fulcio leaf certificate from vs with no tlog entry, to the image at tag.
func signLegacyKeyless(t *testing.T, vs *ca.VirtualSigstore, repo name.Repository, tag string) {
	t.Helper()
	leaf, priv, err := vs.GenerateLeafCert(offlineTestSubject, offlineTestIssuer)
	require.NoError(t, err)
	ref, err := name.ParseReference(repo.Tag(tag).String())
	require.NoError(t, err)
	se, err := ociremote.SignedEntity(ref)
	require.NoError(t, err)
	d, err := se.(interface{ Digest() (v1.Hash, error) }).Digest()
	require.NoError(t, err)

	payloadBytes, err := (&payload.Cosign{Image: repo.Digest(d.String())}).MarshalJSON()
	require.NoError(t, err)
	signer, err := signature.LoadSignerVerifier(priv, crypto.SHA256)
	require.NoError(t, err)
	sig, err := signer.SignMessage(bytes.NewReader(payloadBytes))
	require.NoError(t, err)
	leafPEM, err := cryptoutils.MarshalCertificateToPEM(leaf)
	require.NoError(t, err)

	ociSig, err := ocistatic.NewSignature(payloadBytes, base64.StdEncoding.EncodeToString(sig), ocistatic.WithCertChain(leafPEM, nil))
	require.NoError(t, err)
	newSE, err := mutate.AttachSignatureToEntity(se, ociSig)
	require.NoError(t, err)
	require.NoError(t, ociremote.WriteSignatures(repo, newSE))
}

// Test_LegacyKeylessSignature_InlineTrustedRoot covers the non-bundle path: a
// keyless .sig signature is checked against the inline root's Fulcio CAs, and
// a root without CAs refuses it instead of falling back to TUF's Fulcio roots.
func Test_LegacyKeylessSignature_InlineTrustedRoot(t *testing.T) {
	vs, trustedRoot := newPrivateSigstore(t)
	repo, _ := newBundleRegistry(t)
	pushSubject(t, repo, "subject")
	signLegacyKeyless(t, vs, repo, "subject")
	img := fetchTestImage(t, repo.Tag("subject").String())

	t.Run("verifies against a root with the issuing CA", func(t *testing.T) {
		tufCalls := stubTufUnreachable(t)
		v := NewVerifier(nil, logr.Discard())
		require.NoError(t, v.VerifyImageSignature(context.TODO(), img, offlineKeylessAttestor(&v1beta1.StringOrExpression{Value: trustedRoot})))
		require.Zero(t, tufCalls.Load())
	})

	t.Run("refused by a root without CAs", func(t *testing.T) {
		tufCalls := stubTufUnreachable(t)
		v := NewVerifier(nil, logr.Discard())
		attestor := offlineKeylessAttestor(&v1beta1.StringOrExpression{Value: composeTrustedRoot(t, nil, vs)})
		err := v.VerifyImageSignature(context.TODO(), img, attestor)
		require.ErrorContains(t, err, "leaf certificate verification failed")
		assert.NotContains(t, err.Error(), "TUF")
		require.Zero(t, tufCalls.Load())
	})
}

// Test_KeyAttestor_TlogRequired_InlineTrustedRootWithoutTlogs covers a key
// attestor that requires a tlog entry with an inline root that has no tlogs:
// it is not a configuration error, but the signature is still refused at
// verification and TUF is never contacted.
func Test_KeyAttestor_TlogRequired_InlineTrustedRootWithoutTlogs(t *testing.T) {
	githubJSON, err := os.ReadFile("testdata/github-trusted-root.json")
	require.NoError(t, err)
	repo, _ := newBundleRegistry(t)
	pushSubject(t, repo, "subject")
	pubPEM := signV2(t, repo, "subject")
	img := fetchTestImage(t, repo.Tag("subject").String())

	tufCalls := stubTufUnreachable(t)
	v := NewVerifier(nil, logr.Discard())
	attestor := &v1beta1.Attestor{Name: "key", Cosign: &v1beta1.Cosign{
		Key:         &v1beta1.Key{Data: pubPEM},
		TrustedRoot: &v1beta1.StringOrExpression{Value: string(githubJSON)},
	}}
	err = v.VerifyImageSignature(context.TODO(), img, attestor)
	require.ErrorContains(t, err, "rekor client not provided")
	assert.NotContains(t, err.Error(), "TUF")
	assert.Zero(t, tufCalls.Load())
}
