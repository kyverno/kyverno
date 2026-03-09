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
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
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
			_, err := factory.GetClient(context.Background(), tt.creds, "", []string{})
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

// trackingSecretLister wraps a secret lister and tracks which secrets were accessed
type trackingSecretLister struct {
	corev1listers.SecretLister
	accessed map[string]bool // tracks namespace/name pairs that were accessed
}

func (t *trackingSecretLister) Secrets(namespace string) corev1listers.SecretNamespaceLister {
	return &trackingSecretNamespaceLister{
		SecretNamespaceLister: t.SecretLister.Secrets(namespace),
		accessed:              t.accessed,
		namespace:             namespace,
	}
}

type trackingSecretNamespaceLister struct {
	corev1listers.SecretNamespaceLister
	accessed  map[string]bool
	namespace string
}

func (t *trackingSecretNamespaceLister) Get(name string) (*corev1.Secret, error) {
	t.accessed[t.namespace+"/"+name] = true
	return t.SecretNamespaceLister.Get(name)
}

func TestRegistryClientFactory_GetClient(t *testing.T) {
	// Setup fake Kubernetes client with secrets
	clientset := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(clientset, 0)
	secretLister := informerFactory.Core().V1().Secrets().Lister()

	// Start informer and wait for cache sync
	stopCh := make(chan struct{})
	defer close(stopCh)
	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	// Create a tracking secret lister
	trackingLister := &trackingSecretLister{
		SecretLister: secretLister,
		accessed:     make(map[string]bool),
	}

	tests := []struct {
		name               string
		creds              *kyvernov1.ImageRegistryCredentials
		resourceNamespace  string
		imagePullSecrets   []string
		expectGlobalClient bool
		expectedSecrets    []string
		description        string
	}{
		{
			name:               "no creds and no imagePullSecrets returns global client",
			creds:              nil,
			resourceNamespace:  "default",
			imagePullSecrets:   nil,
			expectGlobalClient: true,
			expectedSecrets:    nil,
			description:        "When no credentials or imagePullSecrets provided, should return global client",
		},
		{
			name: "explicit secrets only",
			creds: &kyvernov1.ImageRegistryCredentials{
				Secrets: []string{"kyverno-secret", "production/prod-secret"},
			},
			resourceNamespace:  "default",
			imagePullSecrets:   nil,
			expectGlobalClient: false,
			expectedSecrets:    []string{config.KyvernoNamespace() + "/kyverno-secret", "production/prod-secret"},
			description:        "When only explicit secrets provided, should create new client with kyverno namespace prefixed secret",
		},
		{
			name:               "imagePullSecrets with resource namespace",
			creds:              nil,
			resourceNamespace:  "production",
			imagePullSecrets:   []string{"prod-secret"},
			expectGlobalClient: false,
			expectedSecrets:    []string{"production/prod-secret"},
			description:        "ImagePullSecrets should be prefixed with resource namespace",
		},
		{
			name:               "imagePullSecrets with empty namespace falls back to Kyverno",
			creds:              nil,
			resourceNamespace:  "",
			imagePullSecrets:   []string{"kyverno-secret"},
			expectGlobalClient: false,
			expectedSecrets:    []string{config.KyvernoNamespace() + "/kyverno-secret"},
			description:        "When resource namespace is empty, should use Kyverno namespace",
		},
		{
			name:               "imagePullSecrets with namespace/name notation",
			creds:              nil,
			resourceNamespace:  "default",
			imagePullSecrets:   []string{"production/prod-secret"},
			expectGlobalClient: false,
			expectedSecrets:    []string{"production/prod-secret"},
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
			expectedSecrets: []string{
				config.KyvernoNamespace() + "/kyverno-secret",
				"production/prod-secret",
			},
			description: "Should merge both explicit secrets and imagePullSecrets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset tracking for each test
			trackingLister.accessed = make(map[string]bool)

			factory := DefaultRegistryClientFactory(&mockRegistryClient{}, trackingLister)

			client, err := factory.GetClient(context.Background(), tt.creds, tt.resourceNamespace, tt.imagePullSecrets)
			assert.NilError(t, err, tt.description)
			assert.Assert(t, client != nil, "client should not be nil")

			// Verify the correct client type was returned
			if tt.expectGlobalClient {
				_, ok := client.(*mockRegistryClient)
				assert.Assert(t, ok, "should return global client when no secrets provided")
			}

			client.FetchImageDescriptor(context.Background(), "test")

			// Verify the correct secrets were accessed
			accessed := trackingLister.accessed
			assert.Equal(t, len(accessed), len(tt.expectedSecrets),
				"should access %d secrets, accessed %d: %v", len(tt.expectedSecrets), len(accessed), accessed)
			for _, expectedSecret := range tt.expectedSecrets {
				assert.Assert(t, accessed[expectedSecret],
					"secret %s should have been accessed, accessed secrets: %v", expectedSecret, accessed)
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

	tests := []struct {
		name              string
		resourceNamespace string
		imagePullSecrets  []string
		expectedSecrets   []string
		description       string
	}{
		{
			name:              "simple secret name gets namespace prefix",
			resourceNamespace: "production",
			imagePullSecrets:  []string{"my-secret"},
			expectedSecrets:   []string{"production/my-secret"},
			description:       "Simple secret name should be prefixed with resource namespace",
		},
		{
			name:              "namespace/name notation preserved",
			resourceNamespace: "default",
			imagePullSecrets:  []string{"production/my-secret"},
			expectedSecrets:   []string{"production/my-secret"},
			description:       "Secrets with namespace/name notation should not be modified",
		},
		{
			name:              "empty namespace uses Kyverno namespace",
			resourceNamespace: "",
			imagePullSecrets:  []string{"my-secret"},
			expectedSecrets:   []string{config.KyvernoNamespace() + "/my-secret"},
			description:       "Empty resource namespace should fallback to Kyverno namespace",
		},
		{
			name:              "mixed formats",
			resourceNamespace: "staging",
			imagePullSecrets:  []string{"local-secret", "production/prod-secret", "another-local"},
			expectedSecrets:   []string{"staging/local-secret", "production/prod-secret", "staging/another-local"},
			description:       "Should handle both simple and namespace/name notation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trackingLister := &trackingSecretLister{
				SecretLister: secretLister,
				accessed:     make(map[string]bool),
			}

			factory := DefaultRegistryClientFactory(&mockRegistryClient{}, trackingLister)

			client, err := factory.GetClient(context.Background(), nil, tt.resourceNamespace, tt.imagePullSecrets)
			assert.NilError(t, err, tt.description)
			assert.Assert(t, client != nil)
			// With imagePullSecrets provided, we should NOT get the global client
			_, isGlobal := client.(*mockRegistryClient)
			assert.Assert(t, !isGlobal, "should create new client with imagePullSecrets, not global client")

			client.FetchImageDescriptor(context.Background(), "test")

			// Verify the correct secrets were accessed
			accessed := trackingLister.accessed
			assert.Equal(t, len(accessed), len(tt.expectedSecrets),
				"should access %d secrets, accessed %d: %v", len(tt.expectedSecrets), len(accessed), accessed)
			for _, expectedSecret := range tt.expectedSecrets {
				assert.Assert(t, accessed[expectedSecret],
					"secret %s should have been accessed, accessed secrets: %v", expectedSecret, accessed)
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
