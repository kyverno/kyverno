package tls

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// generateTestCertificate creates a self-signed certificate for testing
func generateTestCertificate() ([]byte, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	return certPEM, nil
}

func TestFetchCert_Success(t *testing.T) {
	// Generate a valid certificate
	certPEM, err := generateTestCertificate()
	assert.NoError(t, err)

	// Create fake secret with certificate data
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-certs",
			Namespace: "kyverno",
		},
		Data: map[string][]byte{
			"ca.pem": certPEM,
		},
	}

	// Create fake Kubernetes client with the secret
	client := fake.NewSimpleClientset(secret)
	ctx := context.Background()

	// Call FetchCert
	creds, err := FetchCert(ctx, "test-certs", client)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, creds)
}

func TestFetchCert_SecretNotFound(t *testing.T) {
	// Create fake Kubernetes client without any secrets
	client := fake.NewSimpleClientset()
	ctx := context.Background()

	// Call FetchCert with non-existent secret name
	creds, err := FetchCert(ctx, "nonexistent-secret", client)

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, creds)
	assert.Contains(t, err.Error(), "error fetching certificate from secret")
}

func TestFetchCert_InvalidCertificateData(t *testing.T) {
	tests := []struct {
		name      string
		secretData map[string][]byte
	}{
		{
			name: "empty ca.pem",
			secretData: map[string][]byte{
				"ca.pem": []byte(""),
			},
		},
		{
			name: "invalid PEM data",
			secretData: map[string][]byte{
				"ca.pem": []byte("not a valid certificate"),
			},
		},
		{
			name: "missing ca.pem key",
			secretData: map[string][]byte{
				"other-key": []byte("some data"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake secret with invalid certificate data
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-certs",
					Namespace: "kyverno",
				},
				Data: tt.secretData,
			}

			client := fake.NewSimpleClientset(secret)
			ctx := context.Background()

			// Call FetchCert
			creds, err := FetchCert(ctx, "test-certs", client)

			// Assertions
			assert.Error(t, err)
			assert.Nil(t, creds)
			assert.Contains(t, err.Error(), "failed to append certificates")
		})
	}
}

func TestFetchCert_ClientError(t *testing.T) {
	// Create fake client that will return an error
	client := fake.NewSimpleClientset()
	client.PrependReactor("get", "secrets", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("simulated client error")
	})

	ctx := context.Background()

	// Call FetchCert
	creds, err := FetchCert(ctx, "test-certs", client)

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, creds)
	assert.Contains(t, err.Error(), "error fetching certificate from secret")
}

func TestFetchCert_MultipleCertificates(t *testing.T) {
	// Generate two valid certificates
	cert1, err := generateTestCertificate()
	assert.NoError(t, err)

	cert2, err := generateTestCertificate()
	assert.NoError(t, err)

	// Concatenate certificates (valid PEM format allows multiple certs)
	multipleCerts := append(cert1, cert2...)

	// Create fake secret with multiple certificates
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-certs",
			Namespace: "kyverno",
		},
		Data: map[string][]byte{
			"ca.pem": multipleCerts,
		},
	}

	client := fake.NewSimpleClientset(secret)
	ctx := context.Background()

	// Call FetchCert
	creds, err := FetchCert(ctx, "test-certs", client)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, creds)
}

func TestFetchCert_EmptySecretName(t *testing.T) {
	// Create fake client
	client := fake.NewSimpleClientset()
	ctx := context.Background()

	// Call FetchCert with empty secret name
	creds, err := FetchCert(ctx, "", client)

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, creds)
}

// TestFetchCert_ContextTimeout verifies behavior with context timeout
// Note: Fake client doesn't fully respect context cancellation, so we test
// that the function properly accepts and passes the context parameter
func TestFetchCert_ContextTimeout(t *testing.T) {
	// Generate a valid certificate
	certPEM, err := generateTestCertificate()
	assert.NoError(t, err)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-certs",
			Namespace: "kyverno",
		},
		Data: map[string][]byte{
			"ca.pem": certPEM,
		},
	}

	client := fake.NewSimpleClientset(secret)

	// Create context with timeout - verifies function signature accepts context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Call FetchCert with timeout context
	creds, err := FetchCert(ctx, "test-certs", client)

	// With a valid secret and sufficient timeout, should succeed
	assert.NoError(t, err)
	assert.NotNil(t, creds)
}
