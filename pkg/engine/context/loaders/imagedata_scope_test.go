package loaders_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	gcrname "github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/context/loaders"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/stretchr/testify/require"
)

func TestImageRegistryContextUsesPolicyDestination(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(registry.New())
	defer server.Close()
	reference := strings.TrimPrefix(server.URL, "http://") + "/policy/context:latest"
	parsed, err := gcrname.ParseReference(reference)
	require.NoError(t, err)
	require.NoError(t, gcrremote.Write(parsed, empty.Image))
	for _, credentials := range []*kyvernov1.ImageRegistryCredentials{nil, {AllowInsecureRegistry: true}} {
		for _, source := range []string{reference, "{{ request.object.spec.containers[0].image }}"} {
			ctx := enginecontext.NewContext(jmespath.New(config.NewDefaultConfiguration(false)))
			require.NoError(t, ctx.AddResource(map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{map[string]interface{}{"name": "source", "image": reference}},
				},
			}))
			client := registryclient.New(registryclient.WithAllowInsecureRegistry(true), registryclient.WithEgressMode("enforce"))
			factory := factories.DefaultRegistryClientFactory(adapters.RegistryClient(client), nil, factories.WithPrivateRegistryEgressMode("enforce"))
			loader := loaders.NewImageDataLoader(context.Background(), logging.GlobalLogger(), kyvernov1.ContextEntry{
				Name: "policyImage",
				ImageRegistry: &kyvernov1.ImageRegistry{
					Reference: source, ImageRegistryCredentials: credentials,
				},
			}, ctx, jmespath.New(config.NewDefaultConfiguration(false)), factory)
			require.NoError(t, loader.LoadData())
			resolved, err := ctx.Query("policyImage.image")
			require.NoError(t, err)
			require.Equal(t, reference, resolved)
			_, err = client.FetchImageDescriptor(context.Background(), reference)
			require.ErrorIs(t, err, registryclient.ErrRegistryAddressBlocked)
		}
	}
}
