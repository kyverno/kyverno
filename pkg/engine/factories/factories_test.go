package factories

import (
	"context"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/stretchr/testify/assert"
)

// TestDefaultContextLoaderFactory verifies the factory is created successfully
func TestDefaultContextLoaderFactory(t *testing.T) {
	factory := DefaultContextLoaderFactory(nil)

	assert.NotNil(t, factory)
}

func TestDefaultContextLoaderFactory_WithNilPolicy(t *testing.T) {
	factory := DefaultContextLoaderFactory(nil)

	loader := factory(nil, kyvernov1.Rule{})

	assert.NotNil(t, loader)
}

func TestDefaultContextLoaderFactory_WithOptions(t *testing.T) {
	factory := DefaultContextLoaderFactory(nil,
		WithAPICallConfig(apicall.APICallConfiguration{}),
	)

	assert.NotNil(t, factory)
}

func TestWithInitializer(t *testing.T) {
	initializer := func(jsonContext enginecontext.Interface) error {
		return nil
	}

	opt := WithInitializer(initializer)

	assert.NotNil(t, opt)
	cl := &contextLoader{}
	opt(cl)
	assert.Len(t, cl.initializers, 1)
}

func TestWithAPICallConfig(t *testing.T) {
	config := apicall.APICallConfiguration{}

	opt := WithAPICallConfig(config)

	assert.NotNil(t, opt)
	// Applying the option should work
	cl := &contextLoader{}
	opt(cl)
	assert.Equal(t, config, cl.apiCallConfig)
}

func TestWithGlobalContextStore(t *testing.T) {
	opt := WithGlobalContextStore(nil)

	assert.NotNil(t, opt)
	// Applying the option should work
	cl := &contextLoader{}
	opt(cl)
	assert.Nil(t, cl.gctxStore)
}

func TestDefaultRegistryClientFactory(t *testing.T) {
	factory := DefaultRegistryClientFactory(nil, nil)

	assert.NotNil(t, factory)
}

func TestRegistryClientFactory_GetClient_NilCreds(t *testing.T) {
	factory := DefaultRegistryClientFactory(nil, nil)

	client, err := factory.GetClient(context.Background(), nil, "", []string{})

	assert.NoError(t, err)
	// When creds are nil, should return the global client (which is nil in this case)
	assert.Nil(t, client)
}

func TestRegistryClientFactory_GetClient_WithCreds(t *testing.T) {
	factory := DefaultRegistryClientFactory(nil, nil)

	creds := &kyvernov1.ImageRegistryCredentials{
		AllowInsecureRegistry: true,
	}

	client, err := factory.GetClient(context.Background(), creds, "", []string{})

	// Should create a new client with the options
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestRegistryClientFactory_GetClient_WithProviders(t *testing.T) {
	factory := DefaultRegistryClientFactory(nil, nil)

	creds := &kyvernov1.ImageRegistryCredentials{
		Providers: []kyvernov1.ImageRegistryCredentialsProvidersType{
			kyvernov1.DEFAULT,
		},
	}

	client, err := factory.GetClient(context.Background(), creds, "", []string{})

	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestContextLoaderFactoryOptions_MultipleOptions(t *testing.T) {
	initializer := func(jsonContext enginecontext.Interface) error {
		return nil
	}

	factory := DefaultContextLoaderFactory(nil,
		WithInitializer(initializer),
		WithAPICallConfig(apicall.APICallConfiguration{}),
		WithGlobalContextStore(nil),
	)

	assert.NotNil(t, factory)
}

func TestContextLoader_Load_EmptyEntries(t *testing.T) {
	factory := DefaultContextLoaderFactory(nil)
	loader := factory(nil, kyvernov1.Rule{})

	err := loader.Load(context.Background(), nil, nil, nil, []kyvernov1.ContextEntry{}, nil)

	assert.NoError(t, err)
}

func TestContextLoader_Load_NilEntries(t *testing.T) {
	factory := DefaultContextLoaderFactory(nil)
	loader := factory(nil, kyvernov1.Rule{})

	err := loader.Load(context.Background(), nil, nil, nil, nil, nil)

	assert.NoError(t, err)
}

func TestRegistryClientFactory_GetClient_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		creds     *kyvernov1.ImageRegistryCredentials
		wantNil   bool
		wantError bool
	}{
		{
			name:      "nil credentials returns global client",
			creds:     nil,
			wantNil:   true,
			wantError: false,
		},
		{
			name: "insecure registry option",
			creds: &kyvernov1.ImageRegistryCredentials{
				AllowInsecureRegistry: true,
			},
			wantNil:   false,
			wantError: false,
		},
		{
			name: "with providers",
			creds: &kyvernov1.ImageRegistryCredentials{
				Providers: []kyvernov1.ImageRegistryCredentialsProvidersType{
					kyvernov1.DEFAULT,
				},
			},
			wantNil:   false,
			wantError: false,
		},
		{
			name: "with secrets configured",
			creds: &kyvernov1.ImageRegistryCredentials{
				Secrets: []string{"secret-1", "secret-2"},
			},
			wantNil:   false,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := DefaultRegistryClientFactory(nil, nil)
			client, err := factory.GetClient(context.Background(), tt.creds, "", []string{})

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.wantNil {
				assert.Nil(t, client)
			} else {
				assert.NotNil(t, client)
			}
		})
	}
}
