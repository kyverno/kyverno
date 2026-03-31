package tls

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
}

func (m *mockClient) Create(_ context.Context, s *corev1.Secret, _ metav1.CreateOptions) (*corev1.Secret, error) {
	return s, nil
}

func (m *mockClient) Update(_ context.Context, s *corev1.Secret, _ metav1.UpdateOptions) (*corev1.Secret, error) {
	return s, nil
}

func (m *mockClient) Delete(_ context.Context, _ string, _ metav1.DeleteOptions) error {
	return nil
}

func generateSelfSignedCert(t *testing.T) (*rsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	return key, cert
}

func TestDecodeTLSSecret_SingleCertificate(t *testing.T) {
	key, cert := generateSelfSignedCert(t)
	keyPem, err := privateKeyToPem(key)
	require.NoError(t, err)
	certPem := certificateToPem(cert)

	mock := &mockClient{
		secrets: map[string]*corev1.Secret{
			"tls-pair": {
				Data: map[string][]byte{
					corev1.TLSCertKey:       certPem,
					corev1.TLSPrivateKeyKey: keyPem,
				},
			},
		},
	}

	renewer := &certRenewer{
		client:     mock,
		pairSecret: "tls-pair",
	}

	_, _, decoded, err := renewer.decodeTLSSecret(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, decoded)
	assert.Equal(t, cert.SerialNumber, decoded.SerialNumber)
}

func TestDecodeTLSSecret_NoCertificates(t *testing.T) {
	key, _ := generateSelfSignedCert(t)
	keyPem, err := privateKeyToPem(key)
	require.NoError(t, err)

	mock := &mockClient{
		secrets: map[string]*corev1.Secret{
			"tls-pair": {
				Data: map[string][]byte{
					corev1.TLSCertKey:       nil,
					corev1.TLSPrivateKeyKey: keyPem,
				},
			},
		},
	}

	renewer := &certRenewer{
		client:     mock,
		pairSecret: "tls-pair",
	}

	_, _, decoded, err := renewer.decodeTLSSecret(context.Background())
	require.NoError(t, err)
	assert.Nil(t, decoded)
}

func TestDecodeTLSSecret_MultipleCertificatesReturnsError(t *testing.T) {
	key, cert1 := generateSelfSignedCert(t)

	// Generate a second certificate
	template2 := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der2, err := x509.CreateCertificate(rand.Reader, template2, template2, &key.PublicKey, key)
	require.NoError(t, err)
	cert2, err := x509.ParseCertificate(der2)
	require.NoError(t, err)

	keyPem, err := privateKeyToPem(key)
	require.NoError(t, err)
	multiCertPem := certificateToPem(cert1, cert2)

	mock := &mockClient{
		secrets: map[string]*corev1.Secret{
			"tls-pair": {
				Data: map[string][]byte{
					corev1.TLSCertKey:       multiCertPem,
					corev1.TLSPrivateKeyKey: keyPem,
				},
			},
		},
	}

	renewer := &certRenewer{
		client:     mock,
		pairSecret: "tls-pair",
	}

	_, _, decoded, err := renewer.decodeTLSSecret(context.Background())
	require.Error(t, err, "decodeTLSSecret should return an error when multiple certificates are found")
	assert.Nil(t, decoded)
	assert.Contains(t, err.Error(), "expected single certificate")
	assert.Contains(t, err.Error(), "got 2")
}
