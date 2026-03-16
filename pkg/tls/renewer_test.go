package tls

import (
	"context"
	"crypto/x509"
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
	err     error
}

func (m *mockClient) Get(_ context.Context, name string, _ metav1.GetOptions) (*corev1.Secret, error) {
	if m.err != nil {
		return nil, m.err
	}
	if secret, exists := m.secrets[name]; exists {
		return secret, nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "secrets"}, name)
}

func (m *mockClient) Create(_ context.Context, secret *corev1.Secret, _ metav1.CreateOptions) (*corev1.Secret, error) {
	return secret, nil
}

func (m *mockClient) Update(_ context.Context, secret *corev1.Secret, _ metav1.UpdateOptions) (*corev1.Secret, error) {
	return secret, nil
}

func (m *mockClient) Delete(_ context.Context, _ string, _ metav1.DeleteOptions) error {
	return nil
}

func TestDecodeTLSSecret(t *testing.T) {
	// Generate a CA and TLS certificate for testing
	caKey, caCert, err := generateCA(nil, 365*24*time.Hour, RSA)
	require.NoError(t, err)

	// Generate a single TLS certificate
	_, tlsCert, err := generateTLS("", caCert, caKey, 24*time.Hour, "test.kyverno.svc", nil, RSA)
	require.NoError(t, err)

	// Generate a second TLS certificate (for the multi-cert case)
	_, tlsCert2, err := generateTLS("", caCert, caKey, 24*time.Hour, "test2.kyverno.svc", nil, RSA)
	require.NoError(t, err)

	singleCertPEM := certificateToPem(tlsCert)
	multiCertPEM := certificateToPem(tlsCert, tlsCert2)

	keyPEM, err := privateKeyToPem(caKey)
	require.NoError(t, err)

	tests := []struct {
		name        string
		secrets     map[string]*corev1.Secret
		clientErr   error
		wantCert    bool
		wantErr     bool
		errContains string
	}{
		{
			name: "zero certs returns nil cert and no error",
			secrets: map[string]*corev1.Secret{
				"tls-pair": {
					ObjectMeta: metav1.ObjectMeta{Name: "tls-pair"},
					Data: map[string][]byte{
						corev1.TLSCertKey:       {},
						corev1.TLSPrivateKeyKey: keyPEM,
					},
				},
			},
			wantCert: false,
			wantErr:  false,
		},
		{
			name: "exactly one cert returns the cert and no error",
			secrets: map[string]*corev1.Secret{
				"tls-pair": {
					ObjectMeta: metav1.ObjectMeta{Name: "tls-pair"},
					Data: map[string][]byte{
						corev1.TLSCertKey:       singleCertPEM,
						corev1.TLSPrivateKeyKey: keyPEM,
					},
				},
			},
			wantCert: true,
			wantErr:  false,
		},
		{
			name: "multiple certs returns descriptive error",
			secrets: map[string]*corev1.Secret{
				"tls-pair": {
					ObjectMeta: metav1.ObjectMeta{Name: "tls-pair"},
					Data: map[string][]byte{
						corev1.TLSCertKey:       multiCertPEM,
						corev1.TLSPrivateKeyKey: keyPEM,
					},
				},
			},
			wantCert:    false,
			wantErr:     true,
			errContains: "expected exactly 1 cert in TLS secret",
		},
		{
			name:    "secret not found returns error",
			secrets: map[string]*corev1.Secret{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &certRenewer{
				client:     &mockClient{secrets: tt.secrets, err: tt.clientErr},
				pairSecret: "tls-pair",
			}

			_, _, cert, err := c.decodeTLSSecret(context.Background())

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}

			if tt.wantCert {
				assert.NotNil(t, cert)
				assert.IsType(t, &x509.Certificate{}, cert)
			} else if !tt.wantErr {
				assert.Nil(t, cert)
			}
		})
	}
}
