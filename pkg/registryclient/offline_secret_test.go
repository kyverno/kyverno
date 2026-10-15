package registryclient

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
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
			_, err := remote.Get(name.MustParseReference("192.0.2.1/image:latest"),
				remote.WithAuthFromKeychain(GuardKeychain(keychain)),
				remote.WithTransport(transport),
				remote.WithContext(context.Background()),
			)
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

func TestOfflinePolicySecretCredentialsRequireBackend(t *testing.T) {
	t.Parallel()
	credentials := policiesv1alpha1.Credentials{Secrets: []string{"tenant/registry"}}
	options, _ := OptsFromImageVerificationCredentials(context.Background(), nil, credentials, "kyverno")
	imageOptions, _ := ImageDataOptsFromImageVerificationCredentials(nil, credentials, "kyverno")
	for label, opts := range map[string][]remote.Option{"registry": options, "image data": imageOptions} {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			transport := &offlineCredentialTransport{}
			opts = append(opts, remote.WithTransport(transport), remote.WithContext(context.Background()))
			_, err := remote.Get(name.MustParseReference("192.0.2.1/image:latest"), opts...)
			require.ErrorContains(t, err, "secret lister is not configured")
			require.Zero(t, transport.calls)
		})
	}
}
