package registryclient

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// mockSecretLister implements corev1listers.SecretNamespaceLister for testing
type mockSecretLister struct {
	secrets map[string]*corev1.Secret
	err     error
}

func (m *mockSecretLister) List(selector labels.Selector) ([]*corev1.Secret, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []*corev1.Secret
	for _, s := range m.secrets {
		result = append(result, s)
	}
	return result, nil
}

func (m *mockSecretLister) Get(name string) (*corev1.Secret, error) {
	if m.err != nil {
		return nil, m.err
	}
	if secret, exists := m.secrets[name]; exists {
		return secret, nil
	}
	return nil, k8serrors.NewNotFound(schema.GroupResource{Group: "", Resource: "secrets"}, name)
}

func newMockSecretLister(secrets map[string]*corev1.Secret, err error) corev1listers.SecretNamespaceLister {
	return &mockSecretLister{secrets: secrets, err: err}
}

func TestGenerateKeychainForPullSecrets_NoSecrets(t *testing.T) {
	lister := newMockSecretLister(map[string]*corev1.Secret{}, nil)

	keychain, err := generateKeychainForPullSecrets(lister)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if keychain == nil {
		t.Error("expected keychain to be non-nil even with no secrets")
	}
}

func TestGenerateKeychainForPullSecrets_WithDockerConfigSecret(t *testing.T) {
	dockerConfig := []byte(`{"auths":{"https://index.docker.io/v1/":{"auth":"dGVzdDp0ZXN0"}}}`)
	secrets := map[string]*corev1.Secret{
		"docker-registry-secret": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "docker-registry-secret",
				Namespace: "default",
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				corev1.DockerConfigJsonKey: dockerConfig,
			},
		},
	}
	lister := newMockSecretLister(secrets, nil)

	keychain, err := generateKeychainForPullSecrets(lister, "docker-registry-secret")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if keychain == nil {
		t.Error("expected keychain to be non-nil")
	}
}

func TestGenerateKeychainForPullSecrets_SecretNotFound(t *testing.T) {
	lister := newMockSecretLister(map[string]*corev1.Secret{}, nil)

	// Should not error when secret doesn't exist - it just skips missing secrets
	keychain, err := generateKeychainForPullSecrets(lister, "nonexistent-secret")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if keychain == nil {
		t.Error("expected keychain to be non-nil")
	}
}

func TestGenerateKeychainForPullSecrets_MultipleSecrets(t *testing.T) {
	secrets := map[string]*corev1.Secret{
		"secret-1": {
			ObjectMeta: metav1.ObjectMeta{Name: "secret-1"},
			Type:       corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				corev1.DockerConfigJsonKey: []byte(`{"auths":{}}`),
			},
		},
		"secret-2": {
			ObjectMeta: metav1.ObjectMeta{Name: "secret-2"},
			Type:       corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				corev1.DockerConfigJsonKey: []byte(`{"auths":{}}`),
			},
		},
	}
	lister := newMockSecretLister(secrets, nil)

	keychain, err := generateKeychainForPullSecrets(lister, "secret-1", "secret-2")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if keychain == nil {
		t.Error("expected keychain to be non-nil")
	}
}

func TestGenerateKeychainForPullSecrets_EmptySecretsList(t *testing.T) {
	lister := newMockSecretLister(map[string]*corev1.Secret{}, nil)

	keychain, err := generateKeychainForPullSecrets(lister)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if keychain == nil {
		t.Error("expected keychain to be non-nil")
	}
}

func TestNewAutoRefreshSecretsKeychain(t *testing.T) {
	lister := newMockSecretLister(map[string]*corev1.Secret{}, nil)

	keychain, err := NewAutoRefreshSecretsKeychain(lister, "my-secret")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if keychain == nil {
		t.Error("expected keychain to be non-nil")
	}
}

func TestNewAutoRefreshSecretsKeychain_NoSecrets(t *testing.T) {
	lister := newMockSecretLister(map[string]*corev1.Secret{}, nil)

	keychain, err := NewAutoRefreshSecretsKeychain(lister)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if keychain == nil {
		t.Error("expected keychain to be non-nil")
	}
}

func TestNewAutoRefreshSecretsKeychain_MultipleSecrets(t *testing.T) {
	lister := newMockSecretLister(map[string]*corev1.Secret{}, nil)

	keychain, err := NewAutoRefreshSecretsKeychain(lister, "secret1", "secret2", "secret3")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if keychain == nil {
		t.Error("expected keychain to be non-nil")
	}
}
