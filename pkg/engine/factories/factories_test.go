package factories

import (
	"context"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
	"github.com/stretchr/testify/assert"
)

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
	initCalled := false
	initializer := func(ctx interface{}) error {
		initCalled = true
		return nil
	}

	opt := WithInitializer(initializer)

	assert.NotNil(t, opt)
	// Applying the option should work
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

	client, err := factory.GetClient(context.Background(), nil)

	assert.NoError(t, err)
	// When creds are nil, should return the global client (which is nil in this case)
	assert.Nil(t, client)
}

func TestRegistryClientFactory_GetClient_WithCreds(t *testing.T) {
	factory := DefaultRegistryClientFactory(nil, nil)

	creds := &kyvernov1.ImageRegistryCredentials{
		AllowInsecureRegistry: true,
	}

	client, err := factory.GetClient(context.Background(), creds)

	// Should create a new client with the options
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestRegistryClientFactory_GetClient_WithProviders(t *testing.T) {
	factory := DefaultRegistryClientFactory(nil, nil)

	creds := &kyvernov1.ImageRegistryCredentials{
		Providers: []kyvernov1.ImageRegistryCredentialsProvidersType{
			kyvernov1.ImageRegistryCredentialsProvidersDefault,
		},
	}

	client, err := factory.GetClient(context.Background(), creds)

	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestContextLoaderFactoryOptions_MultipleOptions(t *testing.T) {
	initCalled := false
	initializer := func(ctx interface{}) error {
		initCalled = true
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
