package tls

import (
	"context"
	"crypto"
	"crypto/rand"
	cryptotls "crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type certRenewerTestClient struct {
	secrets map[string]*corev1.Secret
}

func (c *certRenewerTestClient) Get(_ context.Context, name string, _ metav1.GetOptions) (*corev1.Secret, error) {
	secret, ok := c.secrets[name]
	if !ok {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
	}
	return secret, nil
}

func (c *certRenewerTestClient) Create(_ context.Context, secret *corev1.Secret, _ metav1.CreateOptions) (*corev1.Secret, error) {
	c.secrets[secret.Name] = secret
	return secret, nil
}

func (c *certRenewerTestClient) Update(_ context.Context, secret *corev1.Secret, _ metav1.UpdateOptions) (*corev1.Secret, error) {
	c.secrets[secret.Name] = secret
	return secret, nil
}

func (c *certRenewerTestClient) Delete(_ context.Context, name string, _ metav1.DeleteOptions) error {
	delete(c.secrets, name)
	return nil
}

func TestCertRenewerValidateCertTLSChain(t *testing.T) {
	rootKey, rootCert, err := generateCA(nil, 365*24*time.Hour, ECDSA)
	require.NoError(t, err)
	intermediateKey, intermediateCert := generateIntermediateCA(t, rootCert, rootKey)

	servingKey, servingCert, err := generateTLS("", intermediateCert, intermediateKey, 24*time.Hour, "serving.kyverno.svc", nil, ECDSA)
	require.NoError(t, err)
	servingChainPEM := certificateToPem(servingCert, intermediateCert)
	servingKeyPEM, err := privateKeyToPem(servingKey)
	require.NoError(t, err)
	_, err = cryptotls.X509KeyPair(servingChainPEM, servingKeyPEM)
	require.NoError(t, err, "a standard serving chain should be accepted by tls.X509KeyPair")

	directServingKey, directServingCert, err := generateTLS("", rootCert, rootKey, 24*time.Hour, "direct.kyverno.svc", nil, ECDSA)
	require.NoError(t, err)

	untrustedKey, untrustedCert, err := generateCA(nil, 365*24*time.Hour, ECDSA)
	require.NoError(t, err)
	untrustedServingKey, untrustedServingCert, err := generateTLS("", untrustedCert, untrustedKey, 24*time.Hour, "untrusted.kyverno.svc", nil, ECDSA)
	require.NoError(t, err)

	malformedIntermediate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("not a certificate")})
	malformedChainPEM := append(certificateToPem(servingCert), malformedIntermediate...)

	tests := []struct {
		name    string
		tls     *corev1.Secret
		want    bool
		wantErr bool
	}{
		{
			name: "valid serving chain",
			tls:  newTLSSecret(t, servingChainPEM, servingKey),
			want: true,
		},
		{
			name: "single serving certificate",
			tls:  newTLSSecret(t, certificateToPem(directServingCert), directServingKey),
			want: true,
		},
		{
			name: "missing intermediate certificate",
			tls:  newTLSSecret(t, certificateToPem(servingCert), servingKey),
			want: false,
		},
		{
			name: "untrusted intermediate certificate",
			tls:  newTLSSecret(t, certificateToPem(untrustedServingCert, untrustedCert), untrustedServingKey),
			want: false,
		},
		{
			name: "malformed intermediate certificate",
			tls:  newTLSSecret(t, malformedChainPEM, servingKey),
			want: false,
		},
		{
			name: "malformed private key",
			tls: &corev1.Secret{
				Data: map[string][]byte{
					corev1.TLSCertKey:       certificateToPem(directServingCert),
					corev1.TLSPrivateKeyKey: []byte("not a private key"),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renewer := &certRenewer{
				client: &certRenewerTestClient{secrets: map[string]*corev1.Secret{
					"ca": {
						Data: map[string][]byte{corev1.TLSCertKey: certificateToPem(rootCert)},
					},
					"tls": tt.tls,
				}},
				caSecret:   "ca",
				pairSecret: "tls",
			}

			valid, err := renewer.ValidateCert(t.Context())
			if tt.wantErr {
				require.Error(t, err)
				require.False(t, valid)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, valid)
		})
	}
}

func newTLSSecret(t *testing.T, certPEM []byte, key crypto.PrivateKey) *corev1.Secret {
	t.Helper()
	keyPEM, err := privateKeyToPem(key)
	require.NoError(t, err)
	return &corev1.Secret{
		Data: map[string][]byte{
			corev1.TLSCertKey:       certPEM,
			corev1.TLSPrivateKeyKey: keyPEM,
		},
	}
}

func generateIntermediateCA(t *testing.T, issuer *x509.Certificate, issuerKey crypto.PrivateKey) (crypto.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, err := generatePrivateKey(ECDSA)
	require.NoError(t, err)
	publicKey, err := getPublicKey(key)
	require.NoError(t, err)

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "intermediate.kyverno.svc"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, issuer, publicKey, issuerKey)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return key, cert
}
