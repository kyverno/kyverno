package tls

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// mockClient implements the client interface for testing
type mockClient struct {
	secrets map[string]*corev1.Secret
}

func (m *mockClient) Get(_ context.Context, name string, _ metav1.GetOptions) (*corev1.Secret, error) {
	if s, ok := m.secrets[name]; ok {
		return s, nil
	}
	return nil, k8serrors.NewNotFound(schema.GroupResource{Group: "", Resource: "secrets"}, name)
}

func (m *mockClient) Create(_ context.Context, s *corev1.Secret, _ metav1.CreateOptions) (*corev1.Secret, error) {
	m.secrets[s.Name] = s
	return s, nil
}

func (m *mockClient) Update(_ context.Context, s *corev1.Secret, _ metav1.UpdateOptions) (*corev1.Secret, error) {
	m.secrets[s.Name] = s
	return s, nil
}

func (m *mockClient) Delete(_ context.Context, name string, _ metav1.DeleteOptions) error {
	delete(m.secrets, name)
	return nil
}

// generateTestCACert creates a self-signed CA certificate and returns its key and cert.
func generateTestCACert(t *testing.T) (crypto.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, cert, err := generateCA(nil, 24*time.Hour, RSA)
	require.NoError(t, err)
	return key, cert
}

// generateTestCACertWithAlgorithm creates a self-signed CA using the specified key algorithm.
func generateTestCACertWithAlgorithm(t *testing.T, alg KeyAlgorithm) (crypto.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, cert, err := generateCA(nil, 24*time.Hour, alg)
	require.NoError(t, err)
	return key, cert
}

// generateTestTLSCert creates a TLS leaf cert signed by the given CA.
func generateTestTLSCert(t *testing.T, caCert *x509.Certificate, caKey crypto.PrivateKey) (crypto.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, cert, err := generateTLS("", caCert, caKey, 24*time.Hour, "kyverno-svc.kyverno.svc", nil, RSA)
	require.NoError(t, err)
	return key, cert
}

// generateTestTLSCertWithAlgorithm creates a TLS leaf cert using the specified key algorithm.
func generateTestTLSCertWithAlgorithm(t *testing.T, caCert *x509.Certificate, caKey crypto.PrivateKey, alg KeyAlgorithm) (crypto.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, cert, err := generateTLS("", caCert, caKey, 24*time.Hour, "kyverno-svc.kyverno.svc", nil, alg)
	require.NoError(t, err)
	return key, cert
}

// assertKeyMatchesCert verifies the private key corresponds to the certificate's public key.
func assertKeyMatchesCert(t *testing.T, key crypto.PrivateKey, cert *x509.Certificate) {
	t.Helper()
	switch k := key.(type) {
	case *rsa.PrivateKey:
		pub, ok := cert.PublicKey.(*rsa.PublicKey)
		require.True(t, ok, "cert public key should be RSA")
		assert.True(t, k.PublicKey.Equal(pub), "RSA private key must match certificate public key")
	case *ecdsa.PrivateKey:
		pub, ok := cert.PublicKey.(*ecdsa.PublicKey)
		require.True(t, ok, "cert public key should be ECDSA")
		assert.True(t, k.PublicKey.Equal(pub), "ECDSA private key must match certificate public key")
	case ed25519.PrivateKey:
		pub, ok := cert.PublicKey.(ed25519.PublicKey)
		require.True(t, ok, "cert public key should be Ed25519")
		assert.True(t, k.Public().(ed25519.PublicKey).Equal(pub), "Ed25519 private key must match certificate public key")
	default:
		t.Fatalf("unsupported key type: %T", key)
	}
}

func newTestRenewer(mc *mockClient) *certRenewer {
	return &certRenewer{
		client:     mc,
		pairSecret: "kyverno-tls-pair",
		caSecret:   "kyverno-tls-ca",
		namespace:  "kyverno",
	}
}

func TestDecodeTLSSecret_SingleCert(t *testing.T) {
	caKey, caCert := generateTestCACert(t)
	tlsKey, tlsCert := generateTestTLSCert(t, caCert, caKey)

	keyPEM, err := privateKeyToPem(tlsKey)
	require.NoError(t, err)
	certPEM := certificateToPem(tlsCert)

	mc := &mockClient{secrets: map[string]*corev1.Secret{
		"kyverno-tls-pair": {
			ObjectMeta: metav1.ObjectMeta{Name: "kyverno-tls-pair", Namespace: "kyverno"},
			Type:       corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       certPEM,
				corev1.TLSPrivateKeyKey: keyPEM,
			},
		},
	}}

	renewer := newTestRenewer(mc)
	secret, key, cert, err := renewer.decodeTLSSecret(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, secret)
	assert.NotNil(t, key)
	assert.NotNil(t, cert)
	assert.Equal(t, tlsCert.SerialNumber, cert.SerialNumber)
	assertKeyMatchesCert(t, key, cert)
}

func TestDecodeTLSSecret_MultiCertChain(t *testing.T) {
	// Simulate what cert-manager does: tls.crt contains leaf + intermediate CA
	caKey, caCert := generateTestCACert(t)
	tlsKey, tlsCert := generateTestTLSCert(t, caCert, caKey)

	keyPEM, err := privateKeyToPem(tlsKey)
	require.NoError(t, err)
	// Bundle leaf + CA (the way cert-manager writes it)
	certPEM := certificateToPem(tlsCert, caCert)

	mc := &mockClient{secrets: map[string]*corev1.Secret{
		"kyverno-tls-pair": {
			ObjectMeta: metav1.ObjectMeta{Name: "kyverno-tls-pair", Namespace: "kyverno"},
			Type:       corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       certPEM,
				corev1.TLSPrivateKeyKey: keyPEM,
			},
		},
	}}

	renewer := newTestRenewer(mc)
	secret, key, cert, err := renewer.decodeTLSSecret(context.Background())
	require.NoError(t, err, "decodeTLSSecret must not error on multi-cert PEM bundle")
	assert.NotNil(t, secret)
	assert.NotNil(t, key)
	assert.NotNil(t, cert, "cert must not be nil when PEM bundle contains multiple certs")
	// The returned cert should be the leaf (first in the chain), not the CA
	assert.Equal(t, tlsCert.SerialNumber, cert.SerialNumber)
	assert.False(t, cert.IsCA, "returned cert should be the leaf, not the CA")
	assertKeyMatchesCert(t, key, cert)
}

func TestDecodeTLSSecret_MultiCertChain_ReversedOrder(t *testing.T) {
	// Bundle with CA first, leaf second — should still return the leaf
	caKey, caCert := generateTestCACert(t)
	tlsKey, tlsCert := generateTestTLSCert(t, caCert, caKey)

	keyPEM, err := privateKeyToPem(tlsKey)
	require.NoError(t, err)
	// CA first, then leaf (reversed order)
	certPEM := certificateToPem(caCert, tlsCert)

	mc := &mockClient{secrets: map[string]*corev1.Secret{
		"kyverno-tls-pair": {
			ObjectMeta: metav1.ObjectMeta{Name: "kyverno-tls-pair", Namespace: "kyverno"},
			Type:       corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       certPEM,
				corev1.TLSPrivateKeyKey: keyPEM,
			},
		},
	}}

	renewer := newTestRenewer(mc)
	secret, key, cert, err := renewer.decodeTLSSecret(context.Background())
	require.NoError(t, err, "decodeTLSSecret must not error on reversed multi-cert PEM bundle")
	assert.NotNil(t, secret)
	assert.NotNil(t, key)
	assert.NotNil(t, cert, "cert must not be nil when PEM bundle contains multiple certs")
	assert.Equal(t, tlsCert.SerialNumber, cert.SerialNumber, "should return the leaf cert regardless of order")
	assert.False(t, cert.IsCA, "returned cert should be the leaf, not the CA")
	assertKeyMatchesCert(t, key, cert)
}

func TestDecodeTLSSecret_MultiCertChain_ECDSA(t *testing.T) {
	// Verify multi-cert handling works with ECDSA P-256 keys (common in production)
	caKey, caCert := generateTestCACertWithAlgorithm(t, ECDSA)
	tlsKey, tlsCert := generateTestTLSCertWithAlgorithm(t, caCert, caKey, ECDSA)

	keyPEM, err := privateKeyToPem(tlsKey)
	require.NoError(t, err)
	certPEM := certificateToPem(tlsCert, caCert)

	mc := &mockClient{secrets: map[string]*corev1.Secret{
		"kyverno-tls-pair": {
			ObjectMeta: metav1.ObjectMeta{Name: "kyverno-tls-pair", Namespace: "kyverno"},
			Type:       corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       certPEM,
				corev1.TLSPrivateKeyKey: keyPEM,
			},
		},
	}}

	renewer := newTestRenewer(mc)
	secret, key, cert, err := renewer.decodeTLSSecret(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, secret)
	assert.NotNil(t, cert)
	assert.False(t, cert.IsCA)
	assert.Equal(t, tlsCert.SerialNumber, cert.SerialNumber)
	assertKeyMatchesCert(t, key, cert)
}

func TestDecodeTLSSecret_MultiCertChain_ECDSA_ReversedOrder(t *testing.T) {
	// ECDSA with reversed PEM order (CA first, leaf second)
	caKey, caCert := generateTestCACertWithAlgorithm(t, ECDSA)
	tlsKey, tlsCert := generateTestTLSCertWithAlgorithm(t, caCert, caKey, ECDSA)

	keyPEM, err := privateKeyToPem(tlsKey)
	require.NoError(t, err)
	certPEM := certificateToPem(caCert, tlsCert)

	mc := &mockClient{secrets: map[string]*corev1.Secret{
		"kyverno-tls-pair": {
			ObjectMeta: metav1.ObjectMeta{Name: "kyverno-tls-pair", Namespace: "kyverno"},
			Type:       corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       certPEM,
				corev1.TLSPrivateKeyKey: keyPEM,
			},
		},
	}}

	renewer := newTestRenewer(mc)
	_, key, cert, err := renewer.decodeTLSSecret(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, cert)
	assert.False(t, cert.IsCA)
	assert.Equal(t, tlsCert.SerialNumber, cert.SerialNumber)
	assertKeyMatchesCert(t, key, cert)
}

func TestDecodeTLSSecret_AllCAs(t *testing.T) {
	// Bundle contains only CA certificates (e.g. root + intermediate CA, no leaf).
	// Should fall back to certs[0] without error.
	caKey, caCert := generateTestCACert(t)
	// Generate a second CA as an "intermediate"
	_, intermediateCert := generateTestCACert(t)

	keyPEM, err := privateKeyToPem(caKey)
	require.NoError(t, err)
	certPEM := certificateToPem(caCert, intermediateCert)

	mc := &mockClient{secrets: map[string]*corev1.Secret{
		"kyverno-tls-pair": {
			ObjectMeta: metav1.ObjectMeta{Name: "kyverno-tls-pair", Namespace: "kyverno"},
			Type:       corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       certPEM,
				corev1.TLSPrivateKeyKey: keyPEM,
			},
		},
	}}

	renewer := newTestRenewer(mc)
	secret, _, cert, err := renewer.decodeTLSSecret(context.Background())
	require.NoError(t, err, "should not error when all certs are CAs")
	assert.NotNil(t, secret)
	assert.NotNil(t, cert, "should fall back to first cert")
	assert.True(t, cert.IsCA, "returned cert should be a CA since all certs are CAs")
	assert.Equal(t, caCert.SerialNumber, cert.SerialNumber, "should return first cert as fallback")
}

func TestDecodeTLSSecret_NoCerts(t *testing.T) {
	caKey, _ := generateTestCACert(t)
	keyPEM, err := privateKeyToPem(caKey)
	require.NoError(t, err)

	mc := &mockClient{secrets: map[string]*corev1.Secret{
		"kyverno-tls-pair": {
			ObjectMeta: metav1.ObjectMeta{Name: "kyverno-tls-pair", Namespace: "kyverno"},
			Type:       corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       nil,
				corev1.TLSPrivateKeyKey: keyPEM,
			},
		},
	}}

	renewer := newTestRenewer(mc)
	secret, key, cert, err := renewer.decodeTLSSecret(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, secret)
	assert.NotNil(t, key)
	assert.Nil(t, cert, "cert should be nil when no certs in secret")
}

func TestDecodeTLSSecret_NotFound(t *testing.T) {
	mc := &mockClient{secrets: map[string]*corev1.Secret{}}
	renewer := newTestRenewer(mc)
	_, _, _, err := renewer.decodeTLSSecret(context.Background())
	assert.Error(t, err)
}
