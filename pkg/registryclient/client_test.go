package registryclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/stretchr/testify/require"
)

func TestNativeClientAllowsAuthorChosenLoopback(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		_, _ = io.WriteString(writer, `{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json"}`)
	}))
	defer server.Close()
	configured := New(WithAllowInsecureRegistry(true))
	image := strings.TrimPrefix(server.URL, "http://") + "/example/image:latest"
	_, err := configured.FetchImageDescriptor(context.Background(), image)
	require.NoError(t, err)
	opts, names, err := configured.Options(context.Background())
	require.NoError(t, err)
	reference, err := name.ParseReference(image, names...)
	require.NoError(t, err)
	_, err = gcrremote.Get(reference, opts...)
	require.NoError(t, err)
}

func TestImageVerificationCredentialOptionsBlockPrivateLiteral(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	opts, nameOpts := ResourceImageOptionsFromCredentials(nil, policiesv1alpha1.Credentials{
		AllowInsecureRegistry: true,
	}, "kyverno")
	reference, err := name.ParseReference("127.0.0.1:5000/example/image:latest", nameOpts...)
	require.NoError(t, err)
	_, err = gcrremote.Get(reference, append(opts, gcrremote.WithContext(ctx))...)
	require.ErrorIs(t, err, ErrRegistryAddressBlocked)
}

func TestSplitList(t *testing.T) {
	t.Parallel()
	require.Equal(t, []string{"one", "two"}, splitList(" one, ,two "))
}

func TestRegistryClientsShareConnectionPools(t *testing.T) {
	t.Parallel()
	first := New(WithEgressMode("enforce"), WithPrivateRegistryAllowlist("registry.internal", "10.0.0.0/8")).(*client)
	second := New(WithEgressMode("enforce"), WithPrivateRegistryAllowlist("10.0.0.0/8", "registry.internal"), WithAllowInsecureRegistry(true), WithKeychain(&contextKeychain{})).(*client)
	require.Same(t, first.transport, second.transport)
	require.NotSame(t, first.Keychain(), second.Keychain())
	require.NotSame(t, first.transport, New().(*client).transport)
}

func TestRegistryInvalidConfigFailsBeforeCredentials(t *testing.T) {
	t.Parallel()
	inner := &contextKeychain{}
	c := New(WithPrivateRegistryAllowlist("*.internal"), WithKeychain(inner)).(*client)
	opts, names := c.resourceImageOptions()
	resource, err := name.ParseReference("8.8.8.8/image:latest", names...)
	require.NoError(t, err)
	_, err = gcrremote.Get(resource, opts...)
	require.Error(t, err)
	require.Zero(t, inner.calls)
}

func TestGuardedKeychainPreservesRequestContext(test *testing.T) {
	test.Parallel()
	inner := &contextKeychain{}
	policy, err := egress.New(egress.Config{Mode: "enforce"})
	require.NoError(test, err)
	keychain := &guardedKeychain{inner: inner, policy: policy}
	resource := name.MustParseReference("192.0.2.1/image:tag").Context()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_, err = keychain.ResolveContext(ctx, resource)
	require.NoError(test, err)
	require.Same(test, ctx, inner.context)
	require.Equal(test, 1, inner.calls)
	cancel()
	_, err = keychain.ResolveContext(ctx, resource)
	require.ErrorIs(test, err, context.Canceled)
	_, err = keychain.Resolve(name.MustParseReference("10.0.0.1/image:tag").Context())
	require.ErrorIs(test, err, ErrRegistryAddressBlocked)
	require.Equal(test, 1, inner.calls)
}

type contextKeychain struct {
	calls   int
	context context.Context
	resolve func(context.Context, authn.Resource) (authn.Authenticator, error)
}

func (keychain *contextKeychain) Resolve(authn.Resource) (authn.Authenticator, error) {
	return nil, errors.New("context-aware keychain was called without context")
}

func (keychain *contextKeychain) ResolveContext(ctx context.Context, resource authn.Resource) (authn.Authenticator, error) {
	keychain.calls++
	keychain.context = ctx
	if keychain.resolve != nil {
		return keychain.resolve(ctx, resource)
	}
	return authn.Anonymous, ctx.Err()
}

type offlineCredentialTransport struct {
	requests []*http.Request
	manifest []byte
}

func (transport *offlineCredentialTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.requests = append(transport.requests, request.Clone(request.Context()))
	if transport.manifest == nil {
		return nil, errors.New("unexpected registry request")
	}
	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{"Content-Type": {"application/vnd.docker.distribution.manifest.v2+json"}},
		Body:          io.NopCloser(strings.NewReader(string(transport.manifest))),
		ContentLength: int64(len(transport.manifest)),
		Request:       request,
	}, nil
}

type trackedRequestBody struct {
	io.Reader
	closed bool
}

func (b *trackedRequestBody) Close() error { b.closed = true; return nil }

func TestInvalidTransportClosesRequestBody(t *testing.T) {
	t.Parallel()
	client := New(WithEgressMode("invalid")).(*client)
	body := &trackedRequestBody{Reader: strings.NewReader("registry upload")}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://registry.example/v2/", body)
	require.NoError(t, err)
	_, err = client.transport.RoundTrip(request)
	require.ErrorIs(t, err, client.configErr)
	require.True(t, body.closed)
}
