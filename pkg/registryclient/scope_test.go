package registryclient

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/sdk/extensions/regcreds"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func useResourceImageClient(test *testing.T, configured *client) {
	test.Helper()
	original := globalClient
	globalClient = configured
	test.Cleanup(func() { globalClient = original })
}

func TestGuardResourceImageOptions(test *testing.T) {
	manifest, err := empty.Image.RawManifest()
	require.NoError(test, err)
	globalKeychain := &contextKeychain{}
	configured := New(WithEgressMode("enforce"), WithKeychain(globalKeychain)).(*client)
	useResourceImageClient(test, configured)
	reference := name.MustParseReference("192.0.2.1/image:tag")
	for _, source := range []string{"authenticator", "keychain", "native Secret provider"} {
		test.Run(source, func(test *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			credentials := &authn.Basic{Username: "caller", Password: "secret"}
			keychain := &contextKeychain{resolve: func(_ context.Context, resource authn.Resource) (authn.Authenticator, error) {
				require.Equal(test, reference.Context().String(), resource.String())
				return credentials, nil
			}}
			var secretContext context.Context
			secretCalls := 0
			var options []remote.Option
			switch source {
			case "authenticator":
				options = []remote.Option{remote.WithAuth(credentials)}
			case "keychain":
				options = []remote.Option{remote.WithAuthFromKeychain(keychain)}
			case "native Secret provider":
				lister := contextualSecretLister{namespace: &contextualSecretNamespaceLister{get: func(lookupContext context.Context, secretName string) (*corev1.Secret, error) {
					secretCalls++
					secretContext = lookupContext
					require.Equal(test, "resource-registry", secretName)
					return &corev1.Secret{
						Type: corev1.SecretTypeDockerConfigJson,
						Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"auths":{"192.0.2.1":{"username":"caller","password":"secret"}}}`)},
					}, nil
				}}}
				options, _ = New(WithSecretLister(lister, "tenant"), WithImagePullSecrets("resource-registry")).(*client).imageDataOptions()
			}
			nativeTransport := &offlineCredentialTransport{}
			options = append(options, remote.WithTransport(nativeTransport), remote.WithContext(context.Background()), remote.WithUserAgent("resource-image-options-test"))
			transport := &offlineCredentialTransport{manifest: manifest}
			configured.transport = transport
			for _, image := range []string{
				"127.0.0.1/image:tag", "[::1]/image:tag", "169.254.169.254/image:tag",
				"metadata.google.internal/image:tag", "10.0.0.1/image:tag",
			} {
				guarded, err := GuardResourceImageOptions(ctx, image, options)
				require.ErrorIs(test, err, ErrRegistryAddressBlocked, image)
				require.Nil(test, guarded)
			}
			guarded, err := GuardResourceImageOptions(ctx, reference.Name(), options)
			require.NoError(test, err)
			require.NotSame(test, &options[0], &guarded[0])
			require.Zero(test, keychain.calls)
			require.Zero(test, secretCalls)
			require.Empty(test, transport.requests)
			_, err = remote.Get(reference, guarded...)
			require.NoError(test, err)
			require.Len(test, transport.requests, 2)
			username, password, ok := transport.requests[1].BasicAuth()
			require.True(test, ok)
			require.Equal(test, credentials.Username, username)
			require.Equal(test, credentials.Password, password)
			for _, request := range transport.requests {
				require.Contains(test, request.Header.Get("User-Agent"), "resource-image-options-test")
				require.Same(test, ctx, request.Context())
			}
			switch source {
			case "keychain":
				require.Equal(test, 1, keychain.calls)
				require.Same(test, ctx, keychain.context)
			case "native Secret provider":
				require.Equal(test, 1, secretCalls)
				require.Same(test, ctx, secretContext)
			}
			require.NoError(test, ctx.Err())
			require.Empty(test, nativeTransport.requests)
			require.Zero(test, globalKeychain.calls)
		})
	}
}

func TestPolicyOptionsPermitLocalRegistryButWorkloadOptionsBlockIt(t *testing.T) {
	manifest, err := empty.Image.RawManifest()
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
		if strings.Contains(request.URL.Path, "/manifests/") {
			_, _ = writer.Write(manifest)
		}
	}))
	defer server.Close()
	keychain := &contextKeychain{}
	configured := New(WithKeychain(keychain)).(*client)
	useResourceImageClient(t, configured)
	native, names := configured.imageDataOptions()
	reference, err := name.ParseReference(strings.TrimPrefix(server.URL, "http://")+"/image:tag", names...)
	require.NoError(t, err)
	_, err = remote.Get(reference, native...)
	require.NoError(t, err)
	require.Positive(t, keychain.calls)
	keychain.calls = 0
	opts, _ := ResourceImageOptions()
	_, err = remote.Get(reference, opts...)
	require.ErrorIs(t, err, ErrRegistryAddressBlocked)
	require.Zero(t, keychain.calls)
}

func TestWorkloadOptionsBlockTokenAndRedirectTargets(t *testing.T) {
	for _, kind := range []string{"token", "redirect"} {
		t.Run(kind, func(t *testing.T) {
			manifest, err := empty.Image.RawManifest()
			require.NoError(t, err)
			var targetCalls atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				targetCalls.Add(1)
				if request.URL.Path == "/token" {
					_, _ = writer.Write([]byte(`{"token":"fixture"}`))
					return
				}
				writer.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
				if strings.Contains(request.URL.Path, "/manifests/") {
					_, _ = writer.Write(manifest)
				}
			}))
			defer target.Close()
			targetURL := strings.Replace(target.URL, "127.0.0.1", "localhost", 1)
			origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if kind == "redirect" {
					http.Redirect(writer, request, targetURL+request.URL.Path, http.StatusTemporaryRedirect)
					return
				}
				if request.Header.Get("Authorization") == "" {
					writer.Header().Set("WWW-Authenticate", `Bearer realm="`+targetURL+`/token",service="fixture"`)
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
				writer.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
				_, _ = writer.Write(manifest)
			}))
			defer origin.Close()
			_, port, err := net.SplitHostPort(origin.Listener.Addr().String())
			require.NoError(t, err)
			host := net.JoinHostPort("192.0.2.1", port)
			original := regcreds.DefaultTransport
			transport := original.Clone()
			transport.Proxy = nil
			transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
				if address == host {
					address = origin.Listener.Addr().String()
				}
				return (&net.Dialer{}).DialContext(ctx, network, address)
			}
			regcreds.DefaultTransport = transport
			t.Cleanup(func() { regcreds.DefaultTransport = original; transport.CloseIdleConnections() })
			configured := New(WithAllowInsecureRegistry(true), WithPrivateRegistryAllowlist(host)).(*client)
			native, names := configured.imageDataOptions()
			reference, err := name.ParseReference(host+"/image:tag", names...)
			require.NoError(t, err)
			_, err = remote.Get(reference, native...)
			require.NoError(t, err)
			require.Positive(t, targetCalls.Load())
			targetCalls.Store(0)
			opts, _ := configured.resourceImageOptions()
			_, err = remote.Get(reference, opts...)
			require.ErrorIs(t, err, ErrRegistryAddressBlocked)
			require.Zero(t, targetCalls.Load())
		})
	}
}
