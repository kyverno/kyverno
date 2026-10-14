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
	client engineapi.RegistryClient
}

func (m *mockRegistryClientFactory) GetClient(context.Context, *kyvernov1.ImageRegistryCredentials, string, []string) (engineapi.RegistryClient, error) {
	return m.client, nil
}

func Test_fetchImageData_NonStringJMESPathDoesNotPanic(t *testing.T) {
	ctx := enginecontext.NewContext(jp)

	entry := kyvernov1.ContextEntry{
		Name: "test",
		ImageRegistry: &kyvernov1.ImageRegistry{
			Reference: "ghcr.io/kyverno/kyverno:latest",
			// A bare variable resolves to the raw (non-string) resolved value,
			// bypassing the usual string-producing substitution path.
			JMESPath: "{{ jmesPathVar }}",
		},
	}
	assert.NilError(t, ctx.AddContextEntry("jmesPathVar", []byte(`5`)))

	loader := NewImageDataLoader(
		context.TODO(),
		logging.GlobalLogger(),
		entry,
		ctx,
		jp,
		&mockRegistryClientFactory{client: &mockRegistryClient{}},
	)

	err := loader.LoadData()
	assert.ErrorContains(t, err, "JMESPath")
}
