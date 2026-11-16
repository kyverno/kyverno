package compiler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyverno/kyverno/pkg/toggle"
	sdklibhttp "github.com/kyverno/sdk/extensions/cel/libs/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTokenContext builds the CEL http context the way getOrBuild does — a token
// decorator around an SDK context — but with an empty blocklist so a loopback
// httptest server is reachable. Tests use it directly instead of the shared
// cached context so they do not leak state into each other.
func newTokenContext(t *testing.T) sdklibhttp.ContextInterface {
	t.Helper()
	raw, err := sdklibhttp.NewHTTPWithBlocklist(nil, nil)
	require.NoError(t, err)
	return &tokenHTTPContext{inner: raw}
}

func writeTokenFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}

// captureAuthServer returns a test server that records the Authorization header
// of the last request it received.
func captureAuthServer(t *testing.T) (*httptest.Server, *string) {
	t.Helper()
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &gotAuth
}

func TestTokenHTTPContext_AttachesTokenOnGetAndPost(t *testing.T) {
	t.Setenv("KYVERNO_SCOPED_TOKEN_PATH", writeTokenFile(t, "unit-token\n"))
	srv, gotAuth := captureAuthServer(t)
	ctx := newTokenContext(t)

	_, err := ctx.Get(srv.URL, nil)
	require.NoError(t, err)
	assert.Equal(t, "Bearer unit-token", *gotAuth, "Get should attach the scoped token")

	_, err = ctx.Post(srv.URL, map[string]string{"k": "v"}, nil)
	require.NoError(t, err)
	assert.Equal(t, "Bearer unit-token", *gotAuth, "Post should attach the scoped token")
}

func TestTokenHTTPContext_ExplicitAuthorizationHeaderWins(t *testing.T) {
	t.Setenv("KYVERNO_SCOPED_TOKEN_PATH", writeTokenFile(t, "unit-token\n"))
	srv, gotAuth := captureAuthServer(t)
	ctx := newTokenContext(t)

	for _, header := range []string{"Authorization", "authorization"} {
		_, err := ctx.Get(srv.URL, map[string]string{header: "Bearer caller-token"})
		require.NoError(t, err)
		assert.Equal(t, "Bearer caller-token", *gotAuth, "caller-supplied %q header must be preserved", header)
	}

	// The caller's map must never be mutated when a token is injected.
	headers := map[string]string{"X-Test": "1"}
	_, err := ctx.Get(srv.URL, headers)
	require.NoError(t, err)
	assert.NotContains(t, headers, "Authorization", "withScopedToken must not mutate the caller's map")
}

func TestTokenHTTPContext_MissingTokenFileSendsNoHeader(t *testing.T) {
	t.Setenv("KYVERNO_SCOPED_TOKEN_PATH", filepath.Join(t.TempDir(), "missing-token"))
	srv, gotAuth := captureAuthServer(t)
	ctx := newTokenContext(t)

	_, err := ctx.Get(srv.URL, nil)
	require.NoError(t, err)
	assert.Equal(t, "", *gotAuth, "a missing token file means no Authorization header and no error")
}

func TestTokenHTTPContext_ClientDerivedContextAttachesToken(t *testing.T) {
	t.Setenv("KYVERNO_SCOPED_TOKEN_PATH", writeTokenFile(t, "unit-token\n"))
	srv, gotAuth := captureAuthServer(t)
	ctx := newTokenContext(t)

	child, err := ctx.Client("")
	require.NoError(t, err)
	require.NotNil(t, child)

	_, err = child.Post(srv.URL, map[string]string{"k": "v"}, nil)
	require.NoError(t, err)
	assert.Equal(t, "Bearer unit-token", *gotAuth, "a Client-derived context should attach the scoped token")
}

// TestNewLazyCELHTTPContext_AttachesToken proves the shared, lazily built
// context returned by NewLazyCELHTTPContext("") wraps the SDK context so the
// token is attached end to end. It resets the shared cache and clears the
// blocklist (loopback is blocked by default) and restores both afterward.
func TestNewLazyCELHTTPContext_AttachesToken(t *testing.T) {
	old := sharedHTTPContext
	sharedHTTPContext = &cachedHTTPContext{}
	t.Cleanup(func() { sharedHTTPContext = old })

	require.NoError(t, toggle.HTTPBlocklist.Parse(""))
	t.Cleanup(func() { toggle.HTTPBlocklist.Reset() })

	t.Setenv("KYVERNO_SCOPED_TOKEN_PATH", writeTokenFile(t, "e2e-token\n"))
	srv, gotAuth := captureAuthServer(t)

	ctx := NewLazyCELHTTPContext("")
	_, err := ctx.Post(srv.URL, map[string]string{"k": "v"}, nil)
	require.NoError(t, err)
	assert.Equal(t, "Bearer e2e-token", *gotAuth)
}
