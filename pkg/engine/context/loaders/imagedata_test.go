package loaders

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockImageDataClient struct {
	data *engineapi.ImageData
	err  error
}

func (m *mockImageDataClient) ForRef(context.Context, string) (*engineapi.ImageData, error) {
	return m.data, m.err
}

func (m *mockImageDataClient) FetchImageDescriptor(context.Context, string) (*gcrremote.Descriptor, error) {
	return nil, nil
}

func (m *mockImageDataClient) Keychain() authn.Keychain {
	return nil
}

func (m *mockImageDataClient) Options(context.Context) ([]gcrremote.Option, []name.Option, error) {
	return nil, nil, nil
}

func (m *mockImageDataClient) NameOptions() []name.Option {
	return nil
}

type mockRegistryClientFactory struct {
	client engineapi.RegistryClient
	err    error
}

func (m *mockRegistryClientFactory) GetClient(context.Context, *kyvernov1.ImageRegistryCredentials, string, []string) (engineapi.RegistryClient, error) {
	return m.client, m.err
}

func TestImageDataLoaderCatchError(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctx := context.Background()
	logger := logr.Discard()

	newLoader := func(catchError bool, factory engineapi.RegistryClientFactory) (enginecontext.Interface, enginecontext.Loader) {
		engineCtx := enginecontext.NewContext(jp)
		entry := kyvernov1.ContextEntry{
			Name: "imageData",
			ImageRegistry: &kyvernov1.ImageRegistry{
				Reference:  "example.com/team/does-not-exist:latest",
				CatchError: catchError,
			},
		}
		return engineCtx, NewImageDataLoader(ctx, logger, entry, engineCtx, jp, factory)
	}

	t.Run("successful lookup exposes failure fields", func(t *testing.T) {
		client := &mockImageDataClient{data: &engineapi.ImageData{
			Image:         "nginx:latest",
			ResolvedImage: "nginx@sha256:12345",
			Manifest:      []byte(`{"foo":"bar"}`),
			Config:        []byte(`{"baz":"qux"}`),
		}}
		engineCtx, loader := newLoader(true, &mockRegistryClientFactory{client: client})

		require.NoError(t, loader.LoadData())
		failed, err := engineCtx.Query("imageData.failed")
		require.NoError(t, err)
		assert.Equal(t, false, failed)
		errorMessage, err := engineCtx.Query("imageData.errorMessage")
		require.NoError(t, err)
		assert.Equal(t, "", errorMessage)
		resolvedImage, err := engineCtx.Query("imageData.resolvedImage")
		require.NoError(t, err)
		assert.Equal(t, "nginx@sha256:12345", resolvedImage)
	})

	t.Run("manifest unknown becomes context data", func(t *testing.T) {
		client := &mockImageDataClient{err: errors.New("MANIFEST_UNKNOWN: requested image not found")}
		engineCtx, loader := newLoader(true, &mockRegistryClientFactory{client: client})

		require.NoError(t, loader.LoadData())
		failed, err := engineCtx.Query("imageData.failed")
		require.NoError(t, err)
		assert.Equal(t, true, failed)
		errorMessage, err := engineCtx.Query("imageData.errorMessage")
		require.NoError(t, err)
		assert.Contains(t, errorMessage, "MANIFEST_UNKNOWN")
		resolvedImage, err := engineCtx.Query("imageData.resolvedImage || ''")
		require.NoError(t, err)
		assert.Equal(t, "", resolvedImage)
	})

	t.Run("manifest unknown remains an error by default", func(t *testing.T) {
		client := &mockImageDataClient{err: errors.New("MANIFEST_UNKNOWN: requested image not found")}
		_, loader := newLoader(false, &mockRegistryClientFactory{client: client})

		err := loader.LoadData()
		require.Error(t, err)
		assert.ErrorContains(t, err, "MANIFEST_UNKNOWN")
	})

	t.Run("registry client error becomes context data", func(t *testing.T) {
		engineCtx, loader := newLoader(true, &mockRegistryClientFactory{err: errors.New("credentials unavailable")})

		require.NoError(t, loader.LoadData())
		failed, err := engineCtx.Query("imageData.failed")
		require.NoError(t, err)
		assert.Equal(t, true, failed)
		errorMessage, err := engineCtx.Query("imageData.errorMessage")
		require.NoError(t, err)
		assert.Contains(t, errorMessage, "credentials unavailable")
	})
}
