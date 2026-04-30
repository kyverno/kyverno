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
)

type mockImageDataClient struct {
	data *engineapi.ImageData
	err  error
}

func (m *mockImageDataClient) ForRef(ctx context.Context, ref string) (*engineapi.ImageData, error) {
	return m.data, m.err
}

func (m *mockImageDataClient) FetchImageDescriptor(context.Context, string) (*gcrremote.Descriptor, error) {
	return nil, nil
}

func (m *mockImageDataClient) Keychain() authn.Keychain {
	return nil
}

func (m *mockImageDataClient) Options(context.Context) ([]gcrremote.Option, error) {
	return nil, nil
}

func (m *mockImageDataClient) NameOptions() []name.Option {
	return nil
}

type mockRegistryClientFactory struct {
	client engineapi.RegistryClient
	err    error
}

func (m *mockRegistryClientFactory) GetClient(ctx context.Context, creds *kyvernov1.ImageRegistryCredentials, ns string, pullSecrets []string) (engineapi.RegistryClient, error) {
	return m.client, m.err
}

func TestImageDataLoader_CatchError(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctx := context.Background()
	logger := logr.Discard()

	t.Run("success case", func(t *testing.T) {
		client := &mockImageDataClient{
			data: &engineapi.ImageData{
				Image:         "nginx:latest",
				ResolvedImage: "nginx@sha256:12345",
				Manifest:      []byte(`{"foo":"bar"}`),
				Config:        []byte(`{"baz":"qux"}`),
			},
		}
		factory := &mockRegistryClientFactory{client: client}
		engineCtx := enginecontext.NewContext(jp)
		entry := kyvernov1.ContextEntry{
			Name: "imageData",
			ImageRegistry: &kyvernov1.ImageRegistry{
				Reference:  "nginx:latest",
				CatchError: true,
			},
		}
		loader := NewImageDataLoader(ctx, logger, entry, engineCtx, jp, factory)
		err := loader.LoadData()
		assert.NoError(t, err)

		failed, err := engineCtx.Query("imageData.failed")
		assert.NoError(t, err)
		assert.Equal(t, false, failed)

		errMsg, err := engineCtx.Query("imageData.errorMessage")
		assert.NoError(t, err)
		assert.Equal(t, "", errMsg)

		ref, err := engineCtx.Query("imageData.image")
		assert.NoError(t, err)
		assert.Equal(t, "nginx:latest", ref)
	})

	t.Run("failure with catchError true", func(t *testing.T) {
		client := &mockImageDataClient{
			err: errors.New("manifest not found"),
		}
		factory := &mockRegistryClientFactory{client: client}
		engineCtx := enginecontext.NewContext(jp)
		entry := kyvernov1.ContextEntry{
			Name: "imageData",
			ImageRegistry: &kyvernov1.ImageRegistry{
				Reference:  "non-existent:latest",
				CatchError: true,
			},
		}
		loader := NewImageDataLoader(ctx, logger, entry, engineCtx, jp, factory)
		err := loader.LoadData()
		assert.NoError(t, err)

		failed, err := engineCtx.Query("imageData.failed")
		assert.NoError(t, err)
		assert.Equal(t, true, failed)

		errMsg, err := engineCtx.Query("imageData.errorMessage")
		assert.NoError(t, err)
		assert.Contains(t, errMsg, "manifest not found")
	})

	t.Run("failure with catchError false", func(t *testing.T) {
		client := &mockImageDataClient{
			err: errors.New("manifest not found"),
		}
		factory := &mockRegistryClientFactory{client: client}
		engineCtx := enginecontext.NewContext(jp)
		entry := kyvernov1.ContextEntry{
			Name: "imageData",
			ImageRegistry: &kyvernov1.ImageRegistry{
				Reference:  "non-existent:latest",
				CatchError: false,
			},
		}
		loader := NewImageDataLoader(ctx, logger, entry, engineCtx, jp, factory)
		err := loader.LoadData()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "manifest not found")
	})

	t.Run("GetClient failure with catchError true", func(t *testing.T) {
		factory := &mockRegistryClientFactory{err: errors.New("credentials error")}
		engineCtx := enginecontext.NewContext(jp)
		entry := kyvernov1.ContextEntry{
			Name: "imageData",
			ImageRegistry: &kyvernov1.ImageRegistry{
				Reference:  "nginx:latest",
				CatchError: true,
			},
		}
		loader := NewImageDataLoader(ctx, logger, entry, engineCtx, jp, factory)
		err := loader.LoadData()
		assert.NoError(t, err)

		failed, err := engineCtx.Query("imageData.failed")
		assert.NoError(t, err)
		assert.Equal(t, true, failed)

		errMsg, err := engineCtx.Query("imageData.errorMessage")
		assert.NoError(t, err)
		assert.Contains(t, errMsg, "credentials error")
	})
}
