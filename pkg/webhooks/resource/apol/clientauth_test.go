package apol

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func newTestCA(t *testing.T) testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return testCA{cert: cert, key: key}
}

func (ca testCA) issue(t *testing.T, cn string, usage x509.ExtKeyUsage) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert
}

func (ca testCA) file(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ca.crt")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.cert.Raw}), 0o600))
	return path
}

func TestClientAuth(t *testing.T) {
	t.Parallel()
	ca, other := newTestCA(t), newTestCA(t)
	auth, err := LoadClientAuth(ca.file(t), []string{"kube-apiserver-kubelet-client"})
	require.NoError(t, err)
	anyName, err := LoadClientAuth(ca.file(t), nil)
	require.NoError(t, err)
	tests := []struct {
		name  string
		auth  *ClientAuth
		certs []*x509.Certificate
		ok    bool
	}{
		{"no certificate", auth, nil, false},
		{"allowed name", auth, []*x509.Certificate{ca.issue(t, "kube-apiserver-kubelet-client", x509.ExtKeyUsageClientAuth)}, true},
		{"other name", auth, []*x509.Certificate{ca.issue(t, "system:node:worker", x509.ExtKeyUsageClientAuth)}, false},
		{"other name without allowlist", anyName, []*x509.Certificate{ca.issue(t, "system:node:worker", x509.ExtKeyUsageClientAuth)}, true},
		{"untrusted CA", auth, []*x509.Certificate{other.issue(t, "kube-apiserver-kubelet-client", x509.ExtKeyUsageClientAuth)}, false},
		{"server certificate", auth, []*x509.Certificate{ca.issue(t, "kube-apiserver-kubelet-client", x509.ExtKeyUsageServerAuth)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequest(http.MethodPost, CombinedPath, nil)
			r.TLS = &tls.ConnectionState{PeerCertificates: tt.certs}
			err := tt.auth.Authenticate(r)
			if tt.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestHandlerRejectsUnauthenticatedCallers(t *testing.T) {
	t.Parallel()
	ca := newTestCA(t)
	auth, err := LoadClientAuth(ca.file(t), nil)
	require.NoError(t, err)
	h := newHandler(t, ownerPolicy()).WithClientAuth(auth)
	rec := httptest.NewRecorder()
	h.HandleCombined(rec, httptest.NewRequest(http.MethodPost, CombinedPath, nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestLoadClientAuthErrors(t *testing.T) {
	t.Parallel()
	_, err := LoadClientAuth(filepath.Join(t.TempDir(), "missing.crt"), nil)
	require.Error(t, err)
	empty := filepath.Join(t.TempDir(), "empty.crt")
	require.NoError(t, os.WriteFile(empty, []byte("not pem"), 0o600))
	_, err = LoadClientAuth(empty, nil)
	require.Error(t, err)
}
