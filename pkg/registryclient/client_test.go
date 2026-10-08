package registryclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/kyverno/kyverno/pkg/utils/egress"

	"github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/require"
)

func TestFetchImageDescriptorBlocksPrivateLiteral(t *testing.T) {
	t.Parallel()
	_, err := New(WithAllowInsecureRegistry(true)).FetchImageDescriptor(
		context.Background(),
		"127.0.0.1:5000/example/image:latest",
	)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrRegistryAddressBlocked), err.Error())
}

func TestImageVerificationCredentialOptionsBlockPrivateLiteral(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	opts, nameOpts := OptsFromImageVerificationCredentials(ctx, nil, policiesv1alpha1.Credentials{
		AllowInsecureRegistry: true,
	}, "kyverno")
	reference, err := name.ParseReference("127.0.0.1:5000/example/image:latest", nameOpts...)
	require.NoError(t, err)
	_, err = gcrremote.Get(reference, opts...)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrRegistryAddressBlocked), err.Error())
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
	require.NotSame(t, first.keychain, second.keychain)
	require.NotSame(t, first.transport, New().(*client).transport)
}

func TestRegistryInvalidConfigFailsBeforeCredentials(t *testing.T) {
	t.Parallel()
	inner := &contextKeychain{}
	c := New(WithPrivateRegistryAllowlist("*.internal"), WithKeychain(inner))
	_, _, err := c.Options(context.Background())
	require.Error(t, err)
	resource, err := name.NewRegistry("8.8.8.8")
	require.NoError(t, err)
	_, err = c.Keychain().Resolve(resource)
	require.Error(t, err)
	require.Zero(t, inner.calls)
}

func TestGuardedKeychainBlocksWithExplicitError(t *testing.T) {
	t.Parallel()
	inner := &contextKeychain{}
	policy, err := egress.New(egress.Config{Mode: "enforce"})
	require.NoError(t, err)
	g := &guardedKeychain{inner: inner, policy: policy}
	resource, err := name.NewRegistry("10.0.0.1")
	require.NoError(t, err)
	_, err = g.ResolveContext(context.Background(), resource)
	require.ErrorIs(t, err, ErrRegistryAddressBlocked)
	require.Zero(t, inner.calls)
}

func TestGuardedKeychainPreservesRequestContext(t *testing.T) {
	t.Parallel()
	policy, err := egress.New(egress.Config{})
	require.NoError(t, err)
	resource, err := name.NewRegistry("8.8.8.8")
	require.NoError(t, err)
	inner := &contextKeychain{}
	g := &guardedKeychain{inner: inner, policy: policy}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = g.ResolveContext(ctx, resource)
	require.NoError(t, err)
	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	require.Equal(t, deadline, inner.deadline)
	require.Equal(t, 1, inner.calls)
	cancel()
	_, err = g.ResolveContext(ctx, resource)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, inner.calls)
}

type contextKeychain struct {
	calls    int
	deadline time.Time
}

func (c *contextKeychain) Resolve(authn.Resource) (authn.Authenticator, error) {
	return nil, errors.New("context-aware keychain was called without context")
}

func (c *contextKeychain) ResolveContext(ctx context.Context, _ authn.Resource) (authn.Authenticator, error) {
	c.calls++
	c.deadline, _ = ctx.Deadline()
	return authn.Anonymous, ctx.Err()
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
