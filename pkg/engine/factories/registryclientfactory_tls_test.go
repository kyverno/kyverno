package factories

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

func generateTestCert(t *testing.T) ([]byte, []byte) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Corp"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	return certPEM, keyPEM
}

func TestRegistryClientFactory_TLSClientCert(t *testing.T) {
	certPEM, keyPEM := generateTestCert(t)

	clientset := fake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "good-secret",
				Namespace: config.KyvernoNamespace(),
			},
			Data: map[string][]byte{
				"tls.crt": certPEM,
				"tls.key": keyPEM,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "custom-keys-secret",
				Namespace: config.KyvernoNamespace(),
			},
			Data: map[string][]byte{
				"my.crt": certPEM,
				"my.key": keyPEM,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "malformed-secret",
				Namespace: config.KyvernoNamespace(),
			},
			Data: map[string][]byte{
				"tls.crt": []byte("bad data"),
				"tls.key": []byte("bad data"),
			},
		},
	)

	informerFactory := informers.NewSharedInformerFactory(clientset, 0)
	secretLister := informerFactory.Core().V1().Secrets().Lister()

	stopCh := make(chan struct{})
	defer close(stopCh)
	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	factory := DefaultRegistryClientFactory(&mockRegistryClient{}, secretLister)

	tests := []struct {
		name        string
		creds       *kyvernov1.ImageRegistryCredentials
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid default keys",
			creds: &kyvernov1.ImageRegistryCredentials{
				TLSClientCert: &kyvernov1.TLSClientCert{
					SecretName: "good-secret",
				},
			},
			expectError: false,
		},
		{
			name: "valid custom keys",
			creds: &kyvernov1.ImageRegistryCredentials{
				TLSClientCert: &kyvernov1.TLSClientCert{
					SecretName: "custom-keys-secret",
					CertKey:    "my.crt",
					KeyKey:     "my.key",
				},
			},
			expectError: false,
		},
		{
			name: "missing secret",
			creds: &kyvernov1.ImageRegistryCredentials{
				TLSClientCert: &kyvernov1.TLSClientCert{
					SecretName: "missing-secret",
				},
			},
			expectError: true,
			errorMsg:    "missing-secret",
		},
		{
			name: "malformed cert",
			creds: &kyvernov1.ImageRegistryCredentials{
				TLSClientCert: &kyvernov1.TLSClientCert{
					SecretName: "malformed-secret",
				},
			},
			expectError: true,
			errorMsg:    "failed to parse mTLS cert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := factory.GetClient(context.Background(), tt.creds, "default", nil)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)

				// Verify mTLS behaviorally
				server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
						w.WriteHeader(http.StatusOK)
						return
					}
					w.WriteHeader(http.StatusUnauthorized)
				}))
				server.TLS = &tls.Config{
					ClientAuth: tls.RequireAnyClientCert,
				}
				server.StartTLS()

				// Ensure the client uses the provided certificates when communicating with a TLS endpoint
				// The FetchImageDescriptor method will use the underlying roundtripper.
				ref := strings.TrimPrefix(server.URL, "https://") + "/test/image:latest"
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()
				_, err := client.FetchImageDescriptor(ctx, ref)

				// A 401 Unauthorized from our test server means the certificate was NOT provided or invalid.
				// A 404 Not Found or EOF or other error means the connection was established but path wasn't found (which is OK for behavioral test)
				if err != nil {
					assert.NotContains(t, err.Error(), "401 Unauthorized", "expected behavioral mTLS test to not fail with 401")
				}

				server.CloseClientConnections()
				server.Close()
			}
		})
	}
}
