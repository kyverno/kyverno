package cosign

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sigstore/cosign/v3/pkg/oci/mutate"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	ocistatic "github.com/sigstore/cosign/v3/pkg/oci/static"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
)

// certExtensionOIDCIssuer is the Fulcio certificate extension that carries the
// OIDC issuer, from cosign/pkg/cosign/certextensions.go. cosign reads the issuer
// from this extension, so a locally issued test certificate needs it for
// identity matching to see an issuer at all.
var certExtensionOIDCIssuer = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1}

// testKeylessIssuer is the issuer written into the local test certificates.
const testKeylessIssuer = "https://token.actions.githubusercontent.com"

// localFulcio is a throwaway CA plus leaf issuer, so the keyless tests do not
// depend on a public certificate whose validity window can expire.
type localFulcio struct {
	rootCert *x509.Certificate
	rootKey  *ecdsa.PrivateKey
}

func newLocalFulcio(t *testing.T) *localFulcio {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	now := time.Now().Truncate(time.Second)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "local-fulcio-root"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return &localFulcio{rootCert: cert, rootKey: key}
}

// issueLeaf returns a certificate chaining to the local root whose single SAN is
// sanURI and whose OIDC issuer extension is issuer.
func (f *localFulcio) issueLeaf(t *testing.T, sanURI, issuer string) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	u, err := url.Parse(sanURI)
	require.NoError(t, err)
	now := time.Now().Truncate(time.Second)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "local-fulcio-leaf"},
		NotBefore:    now.Add(-30 * time.Minute),
		NotAfter:     now.Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		URIs:         []*url.URL{u},
		ExtraExtensions: []pkix.Extension{{
			Id: certExtensionOIDCIssuer,
			// cosign reads this extension's raw bytes as the issuer string
			// (CertExtensions.GetIssuer), so it must not be ASN.1-wrapped.
			Value: []byte(issuer),
		}},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, f.rootCert, &key.PublicKey, f.rootKey)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert, key
}

// signKeylessV2 attaches a cosign v2 simple-signing signature whose certificate
// is leaf, so verification has to walk the certificate policy to decide.
func signKeylessV2(t *testing.T, repo name.Repository, tag string, leaf *x509.Certificate, key *ecdsa.PrivateKey) {
	t.Helper()
	ref, err := name.ParseReference(repo.Tag(tag).String())
	require.NoError(t, err)
	se, err := ociremote.SignedEntity(ref)
	require.NoError(t, err)
	d, err := se.(interface{ Digest() (v1.Hash, error) }).Digest()
	require.NoError(t, err)

	payloadBytes, err := (&payload.Cosign{Image: repo.Digest(d.String())}).MarshalJSON()
	require.NoError(t, err)
	signer, err := signature.LoadECDSASignerVerifier(key, crypto.SHA256)
	require.NoError(t, err)
	sig, err := signer.SignMessage(bytes.NewReader(payloadBytes))
	require.NoError(t, err)

	ociSig, err := ocistatic.NewSignature(
		payloadBytes,
		base64.StdEncoding.EncodeToString(sig),
		ocistatic.WithCertChain(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leaf.Raw}), nil),
	)
	require.NoError(t, err)
	newSE, err := mutate.AttachSignatureToEntity(se, ociSig)
	require.NoError(t, err)
	require.NoError(t, ociremote.WriteSignatures(repo, newSE))
}

// Test_KeylessSubjectRegExp_DecidesTheClassicPath is the end-to-end guard for
// the fix: a locally issued certificate whose SAN either matches or does not
// match the resolved subject regexp must be accepted or rejected by the classic
// signature path. It does not use public certificates, whose validity window
// would eventually make both outcomes fail for an unrelated reason.
func Test_KeylessSubjectRegExp_DecidesTheClassicPath(t *testing.T) {
	fulcio := newLocalFulcio(t)
	// Serve the local CA as the Fulcio root and drop the stubbed trusted root, so
	// verification chains the local leaf up to it through cosign's legacy path.
	// The trusted root from TUF is what normally decides this, and its test
	// fixture is a different CA.
	stubTufWithLocalFulcio(t, fulcio.rootCert)

	tests := []struct {
		name           string
		sanURI         string
		subjectRegExp  string
		wantVerifyFail bool
	}{
		{
			name:          "matching SAN is accepted",
			sanURI:        "https://github.com/allowed/repo/.github/workflows/build.yml@refs/heads/main",
			subjectRegExp: `^https://github\.com/allowed/repo/.*$`,
		},
		{
			name:           "non-matching SAN is rejected",
			sanURI:         "https://github.com/other/repo/.github/workflows/build.yml@refs/heads/main",
			subjectRegExp:  `^https://github\.com/allowed/repo/.*$`,
			wantVerifyFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, _ := newBundleRegistry(t)
			pushSubject(t, repo, "subject")
			leaf, key := fulcio.issueLeaf(t, tt.sanURI, testKeylessIssuer)
			signKeylessV2(t, repo, "subject", leaf, key)
			img := fetchTestImage(t, repo.Tag("subject").String())

			// The subject regexp below stands in for whatever the attestor
			// evaluator resolved from subjectExpression; checkOptions copies it
			// through unchanged, which is what this test then verifies.
			attestor := &v1beta1.Attestor{
				Name: "keyless",
				Cosign: &v1beta1.Cosign{
					Keyless: &v1beta1.Keyless{
						Identities: []v1beta1.Identity{{
							Issuer:        testKeylessIssuer,
							SubjectRegExp: tt.subjectRegExp,
						}},
						Roots: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: fulcio.rootCert.Raw})),
					},
					CTLog: &v1beta1.CTLog{InsecureIgnoreTlog: true, InsecureIgnoreSCT: true},
				},
			}

			cOpts, err := checkOptions(context.TODO(), attestor.Cosign, img.RemoteOpts(), img.NameOpts(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.subjectRegExp, cOpts.Identities[0].SubjectRegExp,
				"the resolved subject regexp must reach the cosign identity")
			require.NotNil(t, cOpts.TrustedMaterial,
				"the local Fulcio CA must be the trust material cosign chains the leaf to")

			err = NewVerifier(nil, logr.Discard()).VerifyImageSignature(context.TODO(), img, attestor)
			if tt.wantVerifyFail {
				require.Error(t, err, "a certificate whose SAN does not match the subject regexp must be rejected")
				// The rejection has to come from the identity check, not from an
				// unrelated failure that would make the case vacuous.
				assert.Contains(t, err.Error(), "none of the expected identities matched",
					"rejection must be caused by the subject regexp, got: %v", err)
			} else {
				require.NoError(t, err, "a certificate whose SAN matches the subject regexp must be accepted")
			}
		})
	}
}

// stubTufWithLocalFulcio replaces the TUF-derived trust material with a trusted
// root whose only Fulcio CA is the local one, so the leaf this test issues chains
// up to it on both the sigstore-go and legacy paths. The transparency logs keep
// the shape the fixture uses, because checkOptions reads Rekor and CTLog keys
// from the trusted root.
func stubTufWithLocalFulcio(t *testing.T, ca *x509.Certificate) {
	t.Helper()
	stubTufWith(t, trustedRootWithFulcioCA(t, ca), nil)
}

// trustedRootWithFulcioCA builds a trusted root with ca as its Fulcio certificate
// authority and one transparency log, mirroring newTestTrustedRoot's shape.
func trustedRootWithFulcioCA(t *testing.T, ca *x509.Certificate) *root.TrustedRoot {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	now := time.Now().Truncate(time.Second)
	// cosign validates the leaf at cert.NotBefore, which x509 truncates to the
	// second, so the CA validity window must start strictly earlier than that or
	// the leaf reads as "not valid yet".
	fulcioCA := &root.FulcioCertificateAuthority{
		Root:                ca,
		ValidityPeriodStart: now.Add(-3 * time.Hour),
		ValidityPeriodEnd:   now.Add(24 * time.Hour),
	}
	tlog := &root.TransparencyLog{
		BaseURL:             "https://rekor.sigstore.dev",
		ID:                  []byte("test-log-id"),
		ValidityPeriodStart: now.Add(-3 * time.Hour),
		ValidityPeriodEnd:   now.Add(24 * time.Hour),
		HashFunc:            crypto.SHA256,
		PublicKey:           &key.PublicKey,
		SignatureHashFunc:   crypto.SHA256,
	}
	tr, err := root.NewTrustedRoot(root.TrustedRootMediaType01,
		[]root.CertificateAuthority{fulcioCA},
		map[string]*root.TransparencyLog{"ctlog": tlog},
		nil,
		map[string]*root.TransparencyLog{"rekor": tlog},
	)
	require.NoError(t, err)
	return tr
}
