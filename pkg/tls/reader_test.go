package tls

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// mockSecretNamespaceLister implements corev1listers.SecretNamespaceLister for testing
type mockSecretNamespaceLister struct {
	secrets map[string]*corev1.Secret
	err     error
}

func (m *mockSecretNamespaceLister) List(selector labels.Selector) ([]*corev1.Secret, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []*corev1.Secret
	for _, s := range m.secrets {
		result = append(result, s)
	}
	return result, nil
}

func (m *mockSecretNamespaceLister) Get(name string) (*corev1.Secret, error) {
	if m.err != nil {
		return nil, m.err
	}
	if secret, exists := m.secrets[name]; exists {
		return secret, nil
	}
	return nil, k8serrors.NewNotFound(schema.GroupResource{Group: "", Resource: "secrets"}, name)
}

func newMockLister(secrets map[string]*corev1.Secret, err error) corev1listers.SecretNamespaceLister {
	return &mockSecretNamespaceLister{secrets: secrets, err: err}
}

func TestReadRootCASecret_WithTLSCertKey(t *testing.T) {
	expectedCert := []byte("-----BEGIN CERTIFICATE-----\ntest-certificate-data\n-----END CERTIFICATE-----")
	secrets := map[string]*corev1.Secret{
		"kyverno-root-ca": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kyverno-root-ca",
				Namespace: "kyverno",
			},
			Data: map[string][]byte{
				corev1.TLSCertKey: expectedCert,
			},
		},
	}
	lister := newMockLister(secrets, nil)

	result, err := ReadRootCASecret("kyverno-root-ca", "kyverno", lister)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != string(expectedCert) {
		t.Errorf("got %q, want %q", string(result), string(expectedCert))
	}
}

func TestReadRootCASecret_WithRootCAKey(t *testing.T) {
	expectedCert := []byte("-----BEGIN CERTIFICATE-----\nroot-ca-data\n-----END CERTIFICATE-----")
	secrets := map[string]*corev1.Secret{
		"kyverno-root-ca": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kyverno-root-ca",
				Namespace: "kyverno",
			},
			Data: map[string][]byte{
				rootCAKey: expectedCert,
			},
		},
	}
	lister := newMockLister(secrets, nil)

	result, err := ReadRootCASecret("kyverno-root-ca", "kyverno", lister)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != string(expectedCert) {
		t.Errorf("got %q, want %q", string(result), string(expectedCert))
	}
}

func TestReadRootCASecret_TLSCertKeyHasPriority(t *testing.T) {
	tlsCert := []byte("tls-cert-data")
	rootCACert := []byte("root-ca-data")
	secrets := map[string]*corev1.Secret{
		"kyverno-root-ca": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kyverno-root-ca",
				Namespace: "kyverno",
			},
			Data: map[string][]byte{
				corev1.TLSCertKey: tlsCert,
				rootCAKey:         rootCACert,
			},
		},
	}
	lister := newMockLister(secrets, nil)

	result, err := ReadRootCASecret("kyverno-root-ca", "kyverno", lister)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != string(tlsCert) {
		t.Errorf("TLSCertKey should have priority, got %q, want %q", string(result), string(tlsCert))
	}
}

func TestReadRootCASecret_SecretNotFound(t *testing.T) {
	lister := newMockLister(map[string]*corev1.Secret{}, nil)

	_, err := ReadRootCASecret("nonexistent-secret", "kyverno", lister)

	if err == nil {
		t.Fatal("expected error for nonexistent secret")
	}
	if !k8serrors.IsNotFound(err) {
		t.Errorf("expected NotFound error, got: %v", err)
	}
}

func TestReadRootCASecret_EmptyData(t *testing.T) {
	secrets := map[string]*corev1.Secret{
		"empty-secret": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "empty-secret",
				Namespace: "kyverno",
			},
			Data: map[string][]byte{},
		},
	}
	lister := newMockLister(secrets, nil)

	_, err := ReadRootCASecret("empty-secret", "kyverno", lister)

	if err == nil {
		t.Fatal("expected error for empty secret data")
	}
	if err.Error() != "root CA certificate not found in secret kyverno/empty-secret" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestReadRootCASecret_NilData(t *testing.T) {
	secrets := map[string]*corev1.Secret{
		"nil-data-secret": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nil-data-secret",
				Namespace: "kyverno",
			},
			Data: nil,
		},
	}
	lister := newMockLister(secrets, nil)

	_, err := ReadRootCASecret("nil-data-secret", "kyverno", lister)

	if err == nil {
		t.Fatal("expected error for nil secret data")
	}
}

func TestReadRootCASecret_ListerError(t *testing.T) {
	listerErr := errors.New("connection refused")
	lister := newMockLister(nil, listerErr)

	_, err := ReadRootCASecret("any-secret", "kyverno", lister)

	if err == nil {
		t.Fatal("expected error from lister")
	}
	if err.Error() != "connection refused" {
		t.Errorf("expected lister error, got: %v", err)
	}
}

func TestReadRootCASecret_EmptyCertificateBytes(t *testing.T) {
	secrets := map[string]*corev1.Secret{
		"empty-cert-secret": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "empty-cert-secret",
				Namespace: "kyverno",
			},
			Data: map[string][]byte{
				corev1.TLSCertKey: []byte{},
				rootCAKey:         []byte{},
			},
		},
	}
	lister := newMockLister(secrets, nil)

	_, err := ReadRootCASecret("empty-cert-secret", "kyverno", lister)

	if err == nil {
		t.Fatal("expected error for empty certificate bytes")
	}
}
