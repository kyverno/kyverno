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

// TestBuildCosignOptions_TSACertChainTooManyIntermediates verifies that a TSA certificate
// chain with more than maxIntermediateCerts (10) intermediate CAs is rejected.
func TestBuildCosignOptions_TSACertChainTooManyIntermediates(t *testing.T) {
	rc, err := registryclient.New()
	assert.NilError(t, err)

	// Build a TSA chain: 1 root + 11 intermediates — exceeds maxIntermediateCerts=10.
	// All intermediates are signed by the root so they each have a distinct Issuer from
	// their own Subject, ensuring splitPEMCertificateChain classifies them as intermediates.
	root, rootKey := generateTestRootCA(t)
	certs := []*x509.Certificate{root}
	for i := 0; i < 11; i++ {
		intermediate, _ := generateTestIntermediateCA(t, int64(i+2), root, rootKey)
		certs = append(certs, intermediate)
	}

	// wrongPubKey is a valid ECDSA key defined in verifier_test.go (same package).
	// Using a key avoids the Fulcio-roots network call in buildCosignOptions.
	opts := verifiers.Options{
		Client:       rc,
		Key:          wrongPubKey,
		IgnoreTlog:   true,
		IgnoreSCT:    true,
		TSACertChain: string(certsToPEM(certs)),
	}

	_, err = buildCosignOptions(context.TODO(), opts)
	assert.ErrorContains(t, err, "TSA certificate chain contains too many intermediate certificates")
}

// TestBuildCosignOptions_TSACertChainAtLimit verifies that a TSA chain with exactly
// maxIntermediateCerts (10) intermediates is not rejected by the length check.
func TestBuildCosignOptions_TSACertChainAtLimit(t *testing.T) {
	rc, err := registryclient.New()
	assert.NilError(t, err)

	// Build a TSA chain: 1 root + exactly 10 intermediates — at the limit, must pass.
	// All intermediates are signed by root to ensure they are classified as intermediates.
	root, rootKey := generateTestRootCA(t)
	certs := []*x509.Certificate{root}
	for i := 0; i < 10; i++ {
		intermediate, _ := generateTestIntermediateCA(t, int64(i+2), root, rootKey)
		certs = append(certs, intermediate)
	}

	opts := verifiers.Options{
		Client:       rc,
		Key:          wrongPubKey,
		IgnoreTlog:   true,
		IgnoreSCT:    true,
		TSACertChain: string(certsToPEM(certs)),
	}

	_, err = buildCosignOptions(context.TODO(), opts)
	if err != nil {
		assert.Assert(t, !strings.Contains(err.Error(), "TSA certificate chain contains too many intermediate certificates"),
			"boundary chain (10 intermediates) must not be rejected by intermediate-count check; got: %v", err)
	}
}

// buildCertChainPEM generates n self-signed CA certs and concatenates their PEM encodings.
// loadCertChain (used in buildCosignOptions) loads all PEM blocks without checking chain
// structure, so this produces a chain of length n for limit testing.
func buildCertChainPEM(t *testing.T, n int) string {
	t.Helper()
	var certs []*x509.Certificate
	for i := 0; i < n; i++ {
		cert, _ := generateTestRootCA(t)
		certs = append(certs, cert)
	}
	return string(certsToPEM(certs))
}

// TestBuildCosignOptions_CertChainTooLong verifies that a certificate chain longer than
// maxIntermediateCerts+1 (11) is rejected when opts.Cert and opts.CertChain are set.
func TestBuildCosignOptions_CertChainTooLong(t *testing.T) {
	rc, err := registryclient.New()
	assert.NilError(t, err)

	leafCert, _ := generateTestRootCA(t)
	certPEM := string(certsToPEM([]*x509.Certificate{leafCert}))

	// 12 certs — exceeds maxIntermediateCerts+1=11.
	opts := verifiers.Options{
		Client:     rc,
		Cert:       certPEM,
		CertChain:  buildCertChainPEM(t, 12),
		IgnoreTlog: true,
		IgnoreSCT:  true,
	}

	_, err = buildCosignOptions(context.TODO(), opts)
	assert.ErrorContains(t, err, "certificate chain too long")
}

// TestBuildCosignOptions_CertChainAtLimit verifies that a certificate chain of exactly
// maxIntermediateCerts+1 (11) entries is not rejected by the length check.
func TestBuildCosignOptions_CertChainAtLimit(t *testing.T) {
	rc, err := registryclient.New()
	assert.NilError(t, err)

	leafCert, _ := generateTestRootCA(t)
	certPEM := string(certsToPEM([]*x509.Certificate{leafCert}))

	// Exactly 11 certs — at the boundary.
	opts := verifiers.Options{
		Client:     rc,
		Cert:       certPEM,
		CertChain:  buildCertChainPEM(t, 11),
		IgnoreTlog: true,
		IgnoreSCT:  true,
	}

	_, err = buildCosignOptions(context.TODO(), opts)
	// May fail for chain-validation reasons but must not fail on the length check.
	if err != nil {
		assert.Assert(t, !strings.Contains(err.Error(), "certificate chain too long"),
			"boundary chain (11 certs) must not be rejected by the length check; got: %v", err)
	}
}
