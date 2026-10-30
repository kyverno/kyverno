package registryclient

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/require"
)

type offlineCredentialTransport struct{ calls int }

func (t *offlineCredentialTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.calls++
	return nil, errors.New("unexpected registry request")
}

func TestOfflineSecretCredentialsFailWithoutRegistryAccess(t *testing.T) {
	t.Parallel()
	for _, reference := range []string{"registry", "tenant/registry"} {
		t.Run(reference, func(t *testing.T) {
			t.Parallel()
			transport := &offlineCredentialTransport{}
			fallback := &credentialCalls{}
			keychain := authn.NewMultiKeychain(NewSecretsKeychain(nil, "kyverno", reference), fallback)
			configured := New(WithKeychain(keychain))
			opts, _ := configured.(*client).optionsWithoutPuller(context.Background())
			opts = append(opts, remote.WithTransport(transport))
			_, err := remote.Get(name.MustParseReference("192.0.2.1/image:latest"), opts...)
			require.ErrorContains(t, err, "secret lister is not configured")
			require.Zero(t, fallback.count)
			require.Zero(t, transport.calls)
		})
	}
}

func TestOfflineSecretKeychainPreservesEmptyAndCanceledRequests(t *testing.T) {
	t.Parallel()
	resource := name.MustParseReference("192.0.2.1/image:latest").Context()
	authenticator, err := authn.Resolve(context.Background(), NewSecretsKeychain(nil, "kyverno"), resource)
	require.NoError(t, err)
	require.Equal(t, authn.Anonymous, authenticator)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = authn.Resolve(ctx, NewSecretsKeychain(nil, "kyverno", "registry"), resource)
	require.ErrorIs(t, err, context.Canceled)
}
