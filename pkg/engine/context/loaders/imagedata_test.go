package loaders

import (
	"context"
	"testing"

	gcrname "github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/logging"
	"gotest.tools/v3/assert"
)

type mockRegistryClient struct{}

func (m *mockRegistryClient) ForRef(_ context.Context, ref string) (*engineapi.ImageData, error) {
	return &engineapi.ImageData{
		Image:      ref,
		Registry:   "registry",
		Repository: "repo",
		Manifest:   []byte(`{}`),
		Config:     []byte(`{}`),
	}, nil
}

func (m *mockRegistryClient) FetchImageDescriptor(context.Context, string) (*gcrremote.Descriptor, error) {
	return nil, nil
}

func (m *mockRegistryClient) Options(context.Context) ([]gcrremote.Option, []gcrname.Option, error) {
	return nil, nil, nil
}

func (m *mockRegistryClient) NameOptions() []gcrname.Option {
	return nil
}

type mockRegistryClientFactory struct {
	client            engineapi.RegistryClient
	credentials       *kyvernov1.ImageRegistryCredentials
	resourceNamespace string
	calls             int
}

func (m *mockRegistryClientFactory) GetClient(_ context.Context, credentials *kyvernov1.ImageRegistryCredentials, resourceNamespace string, _ []string) (engineapi.RegistryClient, error) {
	m.calls++
	m.credentials = credentials
	m.resourceNamespace = resourceNamespace
	return m.client, nil
}

func TestImageDataLoaderConfinesCredentialSecrets(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		policyNamespace string
		secrets         []string
		expected        []string
		errorContains   string
	}{
		{
			name:            "bare secret defaults to policy namespace",
			policyNamespace: "tenant-a",
			secrets:         []string{"pull-secret"},
			expected:        []string{"tenant-a/pull-secret"},
		},
		{
			name:            "same namespace is allowed",
			policyNamespace: "tenant-a",
			secrets:         []string{"tenant-a/pull-secret"},
			expected:        []string{"tenant-a/pull-secret"},
		},
		{
			name:            "foreign namespace is rejected",
			policyNamespace: "tenant-a",
			secrets:         []string{"tenant-b/pull-secret"},
			errorContains:   "instead of policy namespace",
		},
		{
			name:            "installation namespace is rejected",
			policyNamespace: "tenant-a",
			secrets:         []string{"kyverno/pull-secret"},
			errorContains:   "instead of policy namespace",
		},
		{
			name:     "cluster policy behavior is preserved",
			secrets:  []string{"pull-secret", "shared/pull-secret"},
			expected: []string{"pull-secret", "shared/pull-secret"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := enginecontext.NewContext(jp)
			factory := &mockRegistryClientFactory{client: &mockRegistryClient{}}
			entry := kyvernov1.ContextEntry{
				Name: "image",
				ImageRegistry: &kyvernov1.ImageRegistry{
					Reference: "ghcr.io/kyverno/kyverno:latest",
					ImageRegistryCredentials: &kyvernov1.ImageRegistryCredentials{
						Secrets: test.secrets,
					},
				},
			}
			loader := NewImageDataLoader(
				context.Background(),
				logging.GlobalLogger(),
				entry,
				ctx,
				jp,
				factory,
				test.policyNamespace,
			)

			err := loader.LoadData()
			if test.errorContains != "" {
				assert.ErrorContains(t, err, test.errorContains)
				assert.Equal(t, 0, factory.calls)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, 1, factory.calls)
			assert.DeepEqual(t, factory.credentials.Secrets, test.expected)
		})
	}
}
