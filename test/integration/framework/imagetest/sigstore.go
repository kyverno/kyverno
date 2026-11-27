package imagetest

import (
	"bytes"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
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
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	// DefaultIssuer is the default OIDC issuer used by virtual Sigstore test helpers.
	DefaultIssuer = "https://issuer.example.com"
	// DefaultSubject is the default OIDC subject used by virtual Sigstore test helpers.
	DefaultSubject = "builder@example.com"
	// DefaultPredicateType is the default in-toto predicate type used when signing v0.3 bundles.
	DefaultPredicateType = "https://slsa.dev/provenance/v1"
)

// VirtualSigstore wraps sigstore-go's in-memory Fulcio CA and TSA for offline keyless testing.
type VirtualSigstore struct {
	*ca.VirtualSigstore
}

// NewVirtualSigstore creates a new ephemeral Sigstore instance (Fulcio CA + TSA).
func NewVirtualSigstore(t *testing.T) *VirtualSigstore {
	t.Helper()
	vs, err := ca.NewVirtualSigstore()
	require.NoError(t, err)
	return &VirtualSigstore{VirtualSigstore: vs}
}

// TrustedRootJSON returns a TrustedRoot JSON containing this instance's Fulcio CAs and TSAs,
// with no transparency or CT log entries.
func (vs *VirtualSigstore) TrustedRootJSON(t *testing.T) string {
	t.Helper()
	return ComposeTrustedRootJSON(t, vs, vs)
}

// ComposeTrustedRootJSON returns a TrustedRoot JSON with the Fulcio CAs of casFrom and the TSAs
// of tsasFrom (either may be nil), and no transparency or CT log entries.
func ComposeTrustedRootJSON(t *testing.T, casFrom, tsasFrom *VirtualSigstore) string {
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
	raw, err := tr.MarshalJSON()
	require.NoError(t, err)
	return string(raw)
}

// SignKeylessBundle builds a Sigstore v0.3 bundle containing a Fulcio leaf certificate and an
// RFC 3161 timestamp (no transparency log entry) over an in-toto statement for subjectDigestHex.
func (vs *VirtualSigstore) SignKeylessBundle(t *testing.T, subjectDigestHex, subject, issuer, predicateType string) []byte {
	t.Helper()
	if predicateType == "" {
		predicateType = DefaultPredicateType
	}
	statement, err := json.Marshal(map[string]any{
		"_type":         "https://in-toto.io/Statement/v1",
		"predicateType": predicateType,
		"subject": []map[string]any{{
			"name":   "subject",
			"digest": map[string]string{"sha256": subjectDigestHex},
		}},
		"predicate": map[string]any{},
	})
	require.NoError(t, err)
	entity, err := vs.Attest(subject, issuer, statement)
	require.NoError(t, err)

	vc, err := entity.VerificationContent()
	require.NoError(t, err)
	cert := vc.Certificate()
	require.NotNil(t, cert)
	sc, err := entity.SignatureContent()
	require.NoError(t, err)
	ec := sc.EnvelopeContent()
	require.NotNil(t, ec)
	envelope := ec.RawEnvelope()
	require.NotNil(t, envelope)
	timestamps, err := entity.Timestamps()
	require.NoError(t, err)
	require.NotEmpty(t, timestamps)

	sigs := make([]*protodsse.Signature, 0, len(envelope.Signatures))
	for _, s := range envelope.Signatures {
		raw, err := base64.StdEncoding.DecodeString(s.Sig)
		require.NoError(t, err)
		sigs = append(sigs, &protodsse.Signature{Sig: raw, Keyid: s.KeyID})
	}
	payloadBytes, err := base64.StdEncoding.DecodeString(envelope.Payload)
	require.NoError(t, err)
	rfc3161 := make([]*protocommon.RFC3161SignedTimestamp, 0, len(timestamps))
	for _, ts := range timestamps {
		rfc3161 = append(rfc3161, &protocommon.RFC3161SignedTimestamp{SignedTimestamp: ts})
	}

	pb := &protobundle.Bundle{
		MediaType: BundleMediaType,
		VerificationMaterial: &protobundle.VerificationMaterial{
			Content: &protobundle.VerificationMaterial_Certificate{
				Certificate: &protocommon.X509Certificate{RawBytes: cert.Raw},
			},
			TimestampVerificationData: &protobundle.TimestampVerificationData{Rfc3161Timestamps: rfc3161},
		},
		Content: &protobundle.Bundle_DsseEnvelope{DsseEnvelope: &protodsse.Envelope{
			Payload:     payloadBytes,
			PayloadType: envelope.PayloadType,
			Signatures:  sigs,
		}},
	}
	raw, err := protojson.Marshal(pb)
	require.NoError(t, err)
	return raw
}

// SignLegacyKeyless attaches a cosign v2 (non-bundle) keyless signature backed by a Fulcio leaf
// certificate from vs (no tlog entry) to the image at repo:tag.
func (vs *VirtualSigstore) SignLegacyKeyless(t *testing.T, repo name.Repository, tag, subject, issuer string) {
	t.Helper()
	leaf, priv, err := vs.GenerateLeafCert(subject, issuer)
	require.NoError(t, err)
	se, err := ociremote.SignedEntity(repo.Tag(tag))
	require.NoError(t, err)
	digester, ok := se.(interface{ Digest() (v1.Hash, error) })
	require.True(t, ok, "signed entity %T does not implement Digest()", se)
	d, err := digester.Digest()
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
