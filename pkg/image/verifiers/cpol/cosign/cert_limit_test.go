package cosign

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/kyverno/kyverno/pkg/image/verifiers"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"gotest.tools/assert"
)

// generateTestRootCA creates a self-signed root CA certificate for testing.
func generateTestRootCA(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test Root CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create root certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("failed to parse root certificate: %v", err)
	}
	return cert, key
}

// generateTestIntermediateCA creates an intermediate CA certificate signed by the given parent.
func generateTestIntermediateCA(t *testing.T, serial int64, parent *x509.Certificate, parentKey *rsa.PrivateKey) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(serial),
		Subject:               pkix.Name{CommonName: "Test Intermediate CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatalf("failed to create intermediate certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("failed to parse intermediate certificate: %v", err)
	}
	return cert, key
}

// certsToPEM encodes a slice of certificates into concatenated PEM blocks.
func certsToPEM(certs []*x509.Certificate) []byte {
	var buf bytes.Buffer
	for _, c := range certs {
		_ = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})
	}
	return buf.Bytes()
}

// repeatPEMBlock concatenates the given PEM block count times. This lets tests
// build arbitrarily long chains without paying RSA key-generation cost per cert.
func repeatPEMBlock(block []byte, count int) []byte {
	var buf bytes.Buffer
	buf.Grow(len(block) * count)
	for i := 0; i < count; i++ {
		buf.Write(block)
	}
	return buf.Bytes()
}

// buildTSAChainPEM returns a PEM with one root CA followed by numIntermediates
// copies of a single intermediate CA PEM block. Repeating the same intermediate
// (rather than regenerating fresh RSA keys per cert) keeps tests fast while still
// producing numIntermediates distinct intermediate blocks for limit checks.
func buildTSAChainPEM(t *testing.T, numIntermediates int) string {
	t.Helper()
	root, rootKey := generateTestRootCA(t)
	intermediate, _ := generateTestIntermediateCA(t, 2, root, rootKey)
	var buf bytes.Buffer
	buf.Write(certsToPEM([]*x509.Certificate{root}))
	buf.Write(repeatPEMBlock(certsToPEM([]*x509.Certificate{intermediate}), numIntermediates))
	return buf.String()
}

// TestBuildCosignOptions_TSACertChainTooManyIntermediates verifies that a TSA certificate
// chain with more than maxIntermediateCerts intermediate CAs is rejected.
func TestBuildCosignOptions_TSACertChainTooManyIntermediates(t *testing.T) {
	rc, err := registryclient.New()
	assert.NilError(t, err)

	tsaChain := buildTSAChainPEM(t, maxIntermediateCerts+1)

	// wrongPubKey is a valid ECDSA key defined in verifier_test.go (same package).
	// Using a key avoids the Fulcio-roots network call in buildCosignOptions.
	opts := verifiers.Options{
		Client:       rc,
		Key:          wrongPubKey,
		IgnoreTlog:   true,
		IgnoreSCT:    true,
		TSACertChain: tsaChain,
	}

	_, err = buildCosignOptions(context.TODO(), opts)
	assert.ErrorContains(t, err, "TSA certificate chain contains too many")
}

// TestBuildCosignOptions_TSACertChainAtLimit verifies that a TSA chain with exactly
// maxIntermediateCerts intermediates is not rejected by the length check.
func TestBuildCosignOptions_TSACertChainAtLimit(t *testing.T) {
	rc, err := registryclient.New()
	assert.NilError(t, err)

	tsaChain := buildTSAChainPEM(t, maxIntermediateCerts)

	opts := verifiers.Options{
		Client:       rc,
		Key:          wrongPubKey,
		IgnoreTlog:   true,
		IgnoreSCT:    true,
		TSACertChain: tsaChain,
	}

	_, err = buildCosignOptions(context.TODO(), opts)
	if err != nil {
		assert.Assert(t, !strings.Contains(err.Error(), "TSA certificate chain contains too many"),
			"boundary chain (%d intermediates) must not be rejected by intermediate-count check; got: %v", maxIntermediateCerts, err)
	}
}

// buildCertChainPEM generates one self-signed cert and concatenates n copies of its
// PEM encoding. loadCertChain (used in buildCosignOptions) loads all PEM blocks without
// checking chain structure, so this produces a chain of length n for limit testing
// without paying RSA key-generation cost per cert.
func buildCertChainPEM(t *testing.T, n int) string {
	t.Helper()
	cert, _ := generateTestRootCA(t)
	return string(repeatPEMBlock(certsToPEM([]*x509.Certificate{cert}), n))
}

// TestBuildCosignOptions_CertChainTooLong verifies that a certificate chain longer than
// maxIntermediateCerts+1 is rejected when opts.Cert and opts.CertChain are set.
func TestBuildCosignOptions_CertChainTooLong(t *testing.T) {
	rc, err := registryclient.New()
	assert.NilError(t, err)

	leafCert, _ := generateTestRootCA(t)
	certPEM := string(certsToPEM([]*x509.Certificate{leafCert}))

	opts := verifiers.Options{
		Client:     rc,
		Cert:       certPEM,
		CertChain:  buildCertChainPEM(t, maxIntermediateCerts+2),
		IgnoreTlog: true,
		IgnoreSCT:  true,
	}

	_, err = buildCosignOptions(context.TODO(), opts)
	assert.ErrorContains(t, err, "certificate chain too long")
}

// TestBuildCosignOptions_CertChainAtLimit verifies that a certificate chain of exactly
// maxIntermediateCerts+1 entries is not rejected by the length check.
func TestBuildCosignOptions_CertChainAtLimit(t *testing.T) {
	rc, err := registryclient.New()
	assert.NilError(t, err)

	leafCert, _ := generateTestRootCA(t)
	certPEM := string(certsToPEM([]*x509.Certificate{leafCert}))

	opts := verifiers.Options{
		Client:     rc,
		Cert:       certPEM,
		CertChain:  buildCertChainPEM(t, maxIntermediateCerts+1),
		IgnoreTlog: true,
		IgnoreSCT:  true,
	}

	_, err = buildCosignOptions(context.TODO(), opts)
	// May fail for chain-validation reasons but must not fail on the length check.
	if err != nil {
		assert.Assert(t, !strings.Contains(err.Error(), "certificate chain too long"),
			"boundary chain (%d certs) must not be rejected by the length check; got: %v", maxIntermediateCerts+1, err)
	}
}
