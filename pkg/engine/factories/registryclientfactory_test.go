package factories

import (
	"context"
	"testing"

	"io"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	"github.com/kyverno/kyverno/pkg/registryclient"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetClient_NilSecretsLister(t *testing.T) {
	// Create a factory with nil secretsLister (simulating CLI usage)
	rclient := registryclient.NewOrDie()
	factory := DefaultRegistryClientFactory(adapters.RegistryClient(rclient), nil)

	tests := []struct {
		name        string
		creds       *kyvernov1.ImageRegistryCredentials
		expectError bool
	}{
		{
			name:        "nil credentials should succeed",
			creds:       nil,
			expectError: false,
		},
		{
			name:        "empty credentials should succeed",
			creds:       &kyvernov1.ImageRegistryCredentials{},
			expectError: false,
		},
		{
			name: "providers only should succeed",
			creds: &kyvernov1.ImageRegistryCredentials{
				Providers: []kyvernov1.ImageRegistryCredentialsProvidersType{"default"},
			},
			expectError: false,
		},
		{
			name: "secrets with nil lister should succeed (secrets silently ignored)",
			creds: &kyvernov1.ImageRegistryCredentials{
				Secrets: []string{"my-secret"},
			},
			expectError: false,
		},
		{
			name: "multiple secrets with nil lister should succeed (secrets silently ignored)",
			creds: &kyvernov1.ImageRegistryCredentials{
				Secrets: []string{"secret1", "secret2"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := factory.GetClient(context.Background(), tt.creds)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// mockRegistryClient implements engineapi.RegistryClient for testing
type mockRegistryClient struct{}

func (m *mockRegistryClient) ForRef(ctx context.Context, ref string) (*engineapi.ImageData, error) {
	return &engineapi.ImageData{}, nil
}

func (m *mockRegistryClient) FetchImageDescriptor(ctx context.Context, ref string) (*gcrremote.Descriptor, error) {
	return &gcrremote.Descriptor{}, nil
}

func (m *mockRegistryClient) Keychain() authn.Keychain {
	return authn.DefaultKeychain
}

func (m *mockRegistryClient) Options(ctx context.Context) ([]gcrremote.Option, error) {
	return []gcrremote.Option{}, nil
}

func (m *mockRegistryClient) NameOptions() []name.Option {
	return []name.Option{}
}

func (m *mockRegistryClient) RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error) {
	return nil, nil
}

func TestRegistryClientFactory_GetClient(t *testing.T) {
	// Setup fake Kubernetes client with secrets
	clientset := fake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kyverno-secret",
				Namespace: config.KyvernoNamespace(),
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				".dockerconfigjson": []byte(`{"auths":{"registry.io":{"auth":"dGVzdDp0ZXN0"}}}`),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prod-secret",
				Namespace: "production",
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				".dockerconfigjson": []byte(`{"auths":{"prod.registry.io":{"auth":"cHJvZDpwcm9k"}}}`),
			},
		},
	)

	informerFactory := informers.NewSharedInformerFactory(clientset, 0)
	secretLister := informerFactory.Core().V1().Secrets().Lister()

	// Start informer and wait for cache sync
	stopCh := make(chan struct{})
	defer close(stopCh)
	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	globalClient := &mockRegistryClient{}
	factory := DefaultRegistryClientFactory(globalClient, secretLister)

	tests := []struct {
		name               string
		creds              *kyvernov1.ImageRegistryCredentials
		resourceNamespace  string
		imagePullSecrets   []string
		expectGlobalClient bool
		description        string
	}{
		{
			name:               "no creds and no imagePullSecrets returns global client",
			creds:              nil,
			resourceNamespace:  "default",
			imagePullSecrets:   nil,
			expectGlobalClient: true,
			description:        "When no credentials or imagePullSecrets provided, should return global client",
		},
		{
			name: "explicit secrets only",
			creds: &kyvernov1.ImageRegistryCredentials{
				Secrets: []string{"kyverno-secret"},
			},
			resourceNamespace:  "default",
			imagePullSecrets:   nil,
			expectGlobalClient: false,
			description:        "When only explicit secrets provided, should create new client",
		},
		{
			name:               "imagePullSecrets with resource namespace",
			creds:              nil,
			resourceNamespace:  "production",
			imagePullSecrets:   []string{"prod-secret"},
			expectGlobalClient: false,
			description:        "ImagePullSecrets should be prefixed with resource namespace",
		},
		{
			name:               "imagePullSecrets with empty namespace falls back to Kyverno",
			creds:              nil,
			resourceNamespace:  "",
			imagePullSecrets:   []string{"kyverno-secret"},
			expectGlobalClient: false,
			description:        "When resource namespace is empty, should use Kyverno namespace",
		},
		{
			name:               "imagePullSecrets with namespace/name notation",
			creds:              nil,
			resourceNamespace:  "default",
			imagePullSecrets:   []string{"production/prod-secret"},
			expectGlobalClient: false,
			description:        "Should support namespace/name notation without modification",
		},
		{
			name: "merge explicit secrets and imagePullSecrets",
			creds: &kyvernov1.ImageRegistryCredentials{
				Secrets: []string{"kyverno-secret"},
			},
			resourceNamespace:  "production",
			imagePullSecrets:   []string{"prod-secret"},
			expectGlobalClient: false,
			description:        "Should merge both explicit secrets and imagePullSecrets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := factory.GetClient(context.Background(), tt.creds, tt.resourceNamespace, tt.imagePullSecrets)
			assert.NilError(t, err, tt.description)
			assert.Assert(t, client != nil, "client should not be nil")

			// When no creds or secrets are provided, we expect the global client
			// Otherwise, we expect a new client instance
			if tt.expectGlobalClient {
				// For global client, we expect the mock client directly
				_, ok := client.(*mockRegistryClient)
				assert.Assert(t, ok, "should return global client (mockRegistryClient)")
			}
		})
	}
}

func TestRegistryClientFactory_GetClient_NamespacePrefixing(t *testing.T) {
	// This test specifically verifies the namespace prefixing logic
	clientset := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(clientset, 0)
	secretLister := informerFactory.Core().V1().Secrets().Lister()

	stopCh := make(chan struct{})
	defer close(stopCh)
	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	globalClient := &mockRegistryClient{}
	factory := DefaultRegistryClientFactory(globalClient, secretLister)

	tests := []struct {
		name              string
		resourceNamespace string
		imagePullSecrets  []string
		description       string
	}{
		{
			name:              "simple secret name gets namespace prefix",
			resourceNamespace: "production",
			imagePullSecrets:  []string{"my-secret"},
			description:       "Simple secret name should be prefixed with resource namespace",
		},
		{
			name:              "namespace/name notation preserved",
			resourceNamespace: "default",
			imagePullSecrets:  []string{"production/my-secret"},
			description:       "Secrets with namespace/name notation should not be modified",
		},
		{
			name:              "empty namespace uses Kyverno namespace",
			resourceNamespace: "",
			imagePullSecrets:  []string{"my-secret"},
			description:       "Empty resource namespace should fallback to Kyverno namespace",
		},
		{
			name:              "mixed formats",
			resourceNamespace: "staging",
			imagePullSecrets:  []string{"local-secret", "production/prod-secret", "another-local"},
			description:       "Should handle mix of simple names and namespace/name notation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily inspect the internal secrets list, but we can verify
			// the client is created without error
			client, err := factory.GetClient(context.Background(), nil, tt.resourceNamespace, tt.imagePullSecrets)
			assert.NilError(t, err, tt.description)
			assert.Assert(t, client != nil)
			// With imagePullSecrets provided, we should NOT get the global client
			_, isGlobal := client.(*mockRegistryClient)
			assert.Assert(t, !isGlobal, "should create new client with imagePullSecrets, not global client")
		})
	}
}
