package imageverify

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/common/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/require"
)

// tokenRegistry wraps an in-memory OCI registry in the bearer token flow used by
// registries such as GitLab, GHCR and Docker Hub: an unauthenticated request is
// challenged, the client exchanges the challenge for a token at the realm, and
// only then is it served. It counts both legs so a test can assert how often the
// handshake is repeated.
type tokenRegistry struct {
	inner  http.Handler
	url    func() string
	pings  atomic.Int64
	tokens atomic.Int64
}

func (t *tokenRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/token" {
		t.tokens.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"token":"test-token"}`)
		return
	}
	if r.URL.Path == "/v2/" {
		t.pings.Add(1)
	}
	if r.Header.Get("Authorization") != "Bearer test-token" {
		w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm=%q,service="registry"`, t.url()+"/token"))
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	t.inner.ServeHTTP(w, r)
}

func (t *tokenRegistry) reset() {
	t.pings.Store(0)
	t.tokens.Store(0)
}

// TestBoundRuntimeReusesRegistryAuth asserts that a bound runtime performs one
// registry auth handshake per repository instead of one per registry call.
//
// Regression test for #17832: the options a policy is compiled with carry a
// keychain and a shared transport but no puller, so go-containerregistry builds a
// fetcher per call and each one re-runs the /v2/ ping and token exchange.
func TestBoundRuntimeReusesRegistryAuth(t *testing.T) {
	handler := &tokenRegistry{inner: registry.New(registry.Logger(log.New(io.Discard, "", 0)))}
	server := httptest.NewServer(handler)
	defer server.Close()
	handler.url = func() string { return server.URL }
	host := strings.TrimPrefix(server.URL, "http://")

	// push a real image so the loader can read a manifest and a config blob
	image := host + "/test/image:tag"
	ref, err := name.ParseReference(image, name.Insecure)
	require.NoError(t, err)
	pushed, err := random.Image(256, 2)
	require.NoError(t, err)
	require.NoError(t, remote.Write(ref, pushed, remote.WithAuth(authn.Anonymous)))

	policy := &policiesv1beta1.ImageValidatingPolicy{
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			Credentials: &policiesv1beta1.Credentials{AllowInsecureRegistry: true},
		},
	}
	factory := NewFactory(logr.Discard(), policy, nil, types.DefaultTypeAdapter, nil)

	// Mirrors how a check actually reaches the registry: the image data loader
	// fetches the manifest and config, then cosign makes many more calls through
	// the options recorded on ImageData (verifier.go uses image.RemoteOpts()).
	exercise := func(authOpts []remote.Option) {
		t.Helper()
		ictx, err := imagedataloader.NewImageContext(nil, nil, nil)
		require.NoError(t, err)
		img, err := ictx.Get(context.Background(), image, authOpts, factory.functions.nameOpts)
		require.NoError(t, err)
		for range 4 {
			_, err := remote.Get(img.NameRef(), img.RemoteOpts()...)
			require.NoError(t, err)
			_, err = remote.Image(img.NameRef(), img.RemoteOpts()...)
			require.NoError(t, err)
		}
	}

	// the options the policy was compiled with: one handshake per call
	handler.reset()
	exercise(factory.functions.authOpts)
	unboundPings, unboundTokens := handler.pings.Load(), handler.tokens.Load()
	require.Greater(t, unboundPings, int64(1),
		"expected the compiled options to re-authenticate per call, otherwise this test proves nothing")

	// the options bound for a request: one handshake for the whole evaluation
	handler.reset()
	exercise(factory.Bind(&Runtime{}).AuthOpts())
	require.Equal(t, int64(1), handler.pings.Load(), "expected a single /v2/ ping for the whole evaluation")
	require.Equal(t, int64(1), handler.tokens.Load(), "expected a single token exchange for the whole evaluation")

	t.Logf("handshakes: compiled options %d pings / %d tokens, bound runtime %d pings / %d tokens",
		unboundPings, unboundTokens, handler.pings.Load(), handler.tokens.Load())
}

// TestBindDoesNotMutateFactoryOptions guards the copy in reuseRegistryAuth: the
// factory is held by a compiled policy and shared by every request bound from it,
// so a puller must never leak back into it and outlive the request.
func TestBindDoesNotMutateFactoryOptions(t *testing.T) {
	policy := &policiesv1beta1.ImageValidatingPolicy{}
	factory := NewFactory(logr.Discard(), policy, nil, types.DefaultTypeAdapter, nil)
	before := len(factory.functions.authOpts)

	first := factory.Bind(&Runtime{}).AuthOpts()
	second := factory.Bind(&Runtime{}).AuthOpts()

	require.Len(t, factory.functions.authOpts, before, "Bind must not append to the factory's options")
	require.Len(t, first, before+1, "the bound options must carry the puller")
	require.Len(t, second, before+1)

	// each request gets its own puller, so a cached token is never shared
	require.NotSame(t, &first[len(first)-1], &second[len(second)-1])
}

// TestZeroRuntimeHasNoAuthOpts covers the fallback in compiledPolicy.registryOpts:
// an unbound runtime must leave the caller on its own options.
func TestZeroRuntimeHasNoAuthOpts(t *testing.T) {
	require.Nil(t, Runtime{}.AuthOpts())
}
