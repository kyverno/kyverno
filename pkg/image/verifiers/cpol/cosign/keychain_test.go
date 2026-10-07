package cosign

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/sdk/extensions/registryclient"
	"gotest.tools/v3/assert"
)

// optionsOnlyClient implements verifiers.Client without exposing credentials.
type optionsOnlyClient struct{}

func (optionsOnlyClient) Options(context.Context) ([]remote.Option, []name.Option, error) {
	return nil, nil, nil
}
func (optionsOnlyClient) NameOptions() []name.Option { return nil }

func TestClientKeychain(t *testing.T) {
	t.Parallel()
	keychain := authn.NewMultiKeychain(authn.DefaultKeychain)
	client := registryclient.New(registryclient.WithKeychain(keychain))
	assert.Assert(t, clientKeychain(client) == client.Keychain(), "the registry client keychain must be used")
	assert.Assert(t, clientKeychain(optionsOnlyClient{}) == nil, "clients without a keychain use their own options")
}
