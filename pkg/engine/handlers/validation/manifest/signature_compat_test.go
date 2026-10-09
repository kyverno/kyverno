package manifest

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/cosign/v3/pkg/cosign/bundle"
	"github.com/sigstore/cosign/v3/pkg/oci/static"
	legacycosign "github.com/sigstore/k8s-manifest-sigstore/pkg/cosign"
	"github.com/sigstore/k8s-manifest-sigstore/pkg/k8smanifest"
	"github.com/sigstore/sigstore/pkg/tuf"
	"github.com/stretchr/testify/require"
)

const (
	manifestBundleSigner = "manifest-signer@example.com"
	manifestBundleIssuer = "https://issuer.example"
)

func TestManifestCertificateBundleCompatibility(t *testing.T) {
	for _, test := range []struct {
		name      string
		expired   bool
		configure func(*testing.T, *manifestBundleFixture, map[string]string, *k8smanifest.VerifyResourceOption)
	}{
		{name: "embedded keyless certificate"},
		{name: "historical certificate at integrated time", expired: true},
		{name: "certificate reference", configure: func(t *testing.T, fixture *manifestBundleFixture, set map[string]string, vo *k8smanifest.VerifyResourceOption) {
			vo.Certificate = writeManifestTrustFile(t, "leaf.pem", fixture.certPEM)
			vo.RootCerts = nil
			delete(set, "certificate")
		}},
		{name: "certificate chain without leaf reference", configure: func(t *testing.T, fixture *manifestBundleFixture, _ map[string]string, vo *k8smanifest.VerifyResourceOption) {
			vo.CertificateChain = writeManifestTrustFile(t, "root.pem", fixture.rootPEM)
			vo.RootCerts = nil
		}},
		{name: "certificate reference and chain", configure: func(t *testing.T, fixture *manifestBundleFixture, set map[string]string, vo *k8smanifest.VerifyResourceOption) {
			vo.Certificate = writeManifestTrustFile(t, "leaf.pem", fixture.certPEM)
			vo.CertificateChain = writeManifestTrustFile(t, "root.pem", fixture.rootPEM)
			vo.RootCerts = nil
			delete(set, "certificate")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newManifestBundleFixture(t, test.expired)
			set, vo := fixture.options(t)
			if test.configure != nil {
				test.configure(t, fixture, set, vo)
			}
			signer, err := verifyBlob(context.Background(), set, "", vo)
			require.NoError(t, err)
			require.Equal(t, manifestBundleSigner, signer)
			if test.configure == nil {
				// Compare the supported offline bundle path with the pinned
				// dependency, using only local trust files and generated keys.
				verified, legacySigner, _, err := legacycosign.VerifyBlob(
					[]byte(set["message"]), []byte(set["signature"]), []byte(set["certificate"]), []byte(set["bundle"]),
					nil, "", "", vo.RekorURL, vo.OIDCIssuer, vo.RootCerts,
				)
				require.NoError(t, err)
				require.True(t, verified)
				require.Equal(t, legacySigner, signer)
			}
		})
	}
}

func TestManifestBundleRejectsTampering(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*testing.T, *manifestBundleFixture, map[string]string)
	}{
		{name: "payload", change: func(t *testing.T, _ *manifestBundleFixture, set map[string]string) {
			set["message"] = manifestAnnotation(t, []byte("modified manifest"))
		}},
		{name: "signature", change: func(_ *testing.T, _ *manifestBundleFixture, set map[string]string) {
			set["signature"] = base64.StdEncoding.EncodeToString([]byte("invalid signature"))
		}},
		{name: "signed entry timestamp", change: func(t *testing.T, fixture *manifestBundleFixture, set map[string]string) {
			entry := fixture.bundle(t, fixture.sig)
			entry.SignedEntryTimestamp[0] ^= 0xff
			data, err := json.Marshal(entry)
			require.NoError(t, err)
			set["bundle"] = manifestAnnotation(t, data)
		}},
		{name: "integrated timestamp", change: func(t *testing.T, fixture *manifestBundleFixture, set map[string]string) {
			entry := fixture.bundle(t, fixture.sig)
			entry.Payload.IntegratedTime++
			data, err := json.Marshal(entry)
			require.NoError(t, err)
			set["bundle"] = manifestAnnotation(t, data)
		}},
		{name: "invalid signature with authentic SET", change: func(t *testing.T, fixture *manifestBundleFixture, set map[string]string) {
			otherKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			require.NoError(t, err)
			hash := sha256.Sum256(fixture.message)
			badSignature, err := ecdsa.SignASN1(rand.Reader, otherKey, hash[:])
			require.NoError(t, err)
			entry := fixture.bundle(t, badSignature)
			data, err := json.Marshal(entry)
			require.NoError(t, err)
			set["signature"] = base64.StdEncoding.EncodeToString(badSignature)
			set["bundle"] = manifestAnnotation(t, data)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newManifestBundleFixture(t, false)
			set, vo := fixture.options(t)
			test.change(t, fixture, set)
			_, err := verifyBlob(context.Background(), set, "", vo)
			require.Error(t, err)
			require.NotErrorIs(t, err, egress.ErrAddressBlocked, "tampering must fail cryptographic verification before any online fallback")
		})
	}
}

func TestManifestBundleRequiresCertificateTrustAndIdentity(t *testing.T) {
	for _, mismatch := range []string{"untrusted certificate authority", "wrong OIDC issuer"} {
		t.Run(mismatch, func(t *testing.T) {
			fixture := newManifestBundleFixture(t, false)
			set, vo := fixture.options(t)
			if mismatch == "untrusted certificate authority" {
				vo.RootCerts = x509.NewCertPool()
			} else {
				vo.OIDCIssuer = "https://different-issuer.example"
			}
			keys := cosign.NewTrustedTransparencyLogPubKeys()
			require.NoError(t, keys.AddTransparencyLogPubKey(fixture.rekorPEM, tuf.Active))
			entry := fixture.bundle(t, fixture.sig)
			sig, err := static.NewSignature(fixture.message, set["signature"], static.WithCertChain(fixture.certPEM, nil), static.WithBundle(entry))
			require.NoError(t, err)
			verified, err := cosign.VerifyBundle(sig, &cosign.CheckOpts{RekorPubKeys: &keys, RootCerts: vo.RootCerts, IgnoreSCT: true, Identities: []cosign.Identity{{Issuer: vo.OIDCIssuer}}})
			require.NoError(t, err)
			require.True(t, verified, "bundle-only verification does not authenticate the certificate issuer or chain")
			_, err = verifyBlob(context.Background(), set, "", vo)
			require.Error(t, err, "the adapter must also verify certificate trust and identity")
			require.NotErrorIs(t, err, egress.ErrAddressBlocked)
		})
	}
}

type manifestBundleFixture struct {
	message        []byte
	sig            []byte
	leafKey        *ecdsa.PrivateKey
	certPEM        []byte
	rootPEM        []byte
	roots          *x509.CertPool
	rekorKey       *ecdsa.PrivateKey
	rekorPEM       []byte
	integratedTime time.Time
}

func newManifestBundleFixture(t *testing.T, expired bool) *manifestBundleFixture {
	t.Helper()
	return newManifestBundleFixtureWithSigner(t, expired, manifestBundleSigner)
}

func newManifestBundleFixtureWithSigner(t *testing.T, expired bool, signer string) *manifestBundleFixture {
	t.Helper()
	newKey := func() *ecdsa.PrivateKey {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		return key
	}
	rootKey, leafKey, rekorKey := newKey(), newKey(), newKey()
	now := time.Now().Truncate(time.Second)
	rootTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "manifest-root"},
		NotBefore: now.Add(-7 * 24 * time.Hour), NotAfter: now.Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &rootKey.PublicKey, rootKey)
	require.NoError(t, err)
	rootCert, err := x509.ParseCertificate(rootDER)
	require.NoError(t, err)
	integratedTime := now
	if expired {
		integratedTime = now.Add(-48 * time.Hour)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2), EmailAddresses: []string{signer},
		NotBefore: integratedTime.Add(-time.Hour), NotAfter: integratedTime.Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		ExtraExtensions: []pkix.Extension{{Id: asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1}, Value: []byte(manifestBundleIssuer)}},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, rootCert, &leafKey.PublicKey, rootKey)
	require.NoError(t, err)
	message := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: signed-manifest\n")
	digest := sha256.Sum256(message)
	sig, err := ecdsa.SignASN1(rand.Reader, leafKey, digest[:])
	require.NoError(t, err)
	rekorDER, err := x509.MarshalPKIXPublicKey(&rekorKey.PublicKey)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(rootCert)
	return &manifestBundleFixture{
		message: message, sig: sig, leafKey: leafKey, roots: roots, rekorKey: rekorKey, integratedTime: integratedTime,
		certPEM:  pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}),
		rootPEM:  pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER}),
		rekorPEM: pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: rekorDER}),
	}
}

func (fixture *manifestBundleFixture) bundle(t *testing.T, sig []byte) *bundle.RekorBundle {
	t.Helper()
	digest := sha256.Sum256(fixture.message)
	body, err := json.Marshal(map[string]interface{}{
		"kind": "hashedrekord", "apiVersion": "0.0.1",
		"spec": map[string]interface{}{
			"data":      map[string]interface{}{"hash": map[string]string{"algorithm": "sha256", "value": hex.EncodeToString(digest[:])}},
			"signature": map[string]interface{}{"content": sig, "publicKey": map[string]interface{}{"content": fixture.certPEM}},
		},
	})
	require.NoError(t, err)
	logID, err := cosign.GetTransparencyLogID(&fixture.rekorKey.PublicKey)
	require.NoError(t, err)
	entry := &bundle.RekorBundle{Payload: bundle.RekorPayload{
		Body: base64.StdEncoding.EncodeToString(body), IntegratedTime: fixture.integratedTime.Unix(), LogIndex: 1, LogID: logID,
	}}
	payload, err := json.Marshal(entry.Payload)
	require.NoError(t, err)
	canonical, err := jsoncanonicalizer.Transform(payload)
	require.NoError(t, err)
	setHash := sha256.Sum256(canonical)
	entry.SignedEntryTimestamp, err = ecdsa.SignASN1(rand.Reader, fixture.rekorKey, setHash[:])
	require.NoError(t, err)
	return entry
}

func (fixture *manifestBundleFixture) options(t *testing.T) (map[string]string, *k8smanifest.VerifyResourceOption) {
	t.Helper()
	keyPath := writeManifestTrustFile(t, "rekor.pub", fixture.rekorPEM)
	t.Setenv("SIGSTORE_REKOR_PUBLIC_KEY", keyPath)
	t.Setenv("SIGSTORE_CT_LOG_PUBLIC_KEY_FILE", keyPath)
	entry, err := json.Marshal(fixture.bundle(t, fixture.sig))
	require.NoError(t, err)
	set := map[string]string{
		"message": manifestAnnotation(t, fixture.message), "signature": base64.StdEncoding.EncodeToString(fixture.sig),
		"certificate": manifestAnnotation(t, fixture.certPEM), "bundle": manifestAnnotation(t, entry),
	}
	// A complete bundle verifies offline. Any unexpected online fallback is
	// rejected by the egress guard instead of reaching an external service.
	vo := &k8smanifest.VerifyResourceOption{}
	vo.RootCerts, vo.OIDCIssuer, vo.RekorURL = fixture.roots, manifestBundleIssuer, "http://127.0.0.1:1"
	return set, vo
}

func manifestAnnotation(t *testing.T, data []byte) string {
	t.Helper()
	var encoded bytes.Buffer
	writer := gzip.NewWriter(&encoded)
	_, err := writer.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return base64.StdEncoding.EncodeToString(encoded.Bytes())
}

func writeManifestTrustFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}
