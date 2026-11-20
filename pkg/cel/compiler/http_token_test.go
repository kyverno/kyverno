package compiler

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyverno/kyverno/pkg/engine/apicall"
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

// slowServer returns a server that answers only after delay, so a call that is
// not bounded by the scoped token client timeout blocks for the whole delay.
func slowServer(t *testing.T, delay time.Duration) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestTokenHTTPContext_HonoursScopedTokenClientTimeout(t *testing.T) {
	t.Setenv("KYVERNO_SCOPED_TOKEN_PATH", writeTokenFile(t, "unit-token\n"))
	srv := slowServer(t, 2*time.Second)

	apicall.SetScopedTokenClientTimeout(200 * time.Millisecond)
	t.Cleanup(func() { apicall.SetScopedTokenClientTimeout(30 * time.Second) })

	ctx := newTokenContext(t)
	// The query string must never show up in the timeout error.
	target := srv.URL + "/?secret=query-value"

	for name, call := range map[string]func() (any, error){
		"Get":  func() (any, error) { return ctx.Get(target, nil) },
		"Post": func() (any, error) { return ctx.Post(target, map[string]string{"k": "v"}, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			_, err := call()
			require.Error(t, err, "a call slower than the configured timeout must fail")
			assert.Contains(t, err.Error(), "timed out after 200ms")
			assert.NotContains(t, err.Error(), "query-value")
			assert.Less(t, time.Since(start), time.Second, "the call must give up long before the server answers")
		})
	}
}

func TestTokenHTTPContext_ClientDerivedContextHonoursTimeout(t *testing.T) {
	t.Setenv("KYVERNO_SCOPED_TOKEN_PATH", writeTokenFile(t, "unit-token\n"))
	srv := slowServer(t, 2*time.Second)

	apicall.SetScopedTokenClientTimeout(200 * time.Millisecond)
	t.Cleanup(func() { apicall.SetScopedTokenClientTimeout(30 * time.Second) })

	ctx := newTokenContext(t)
	child, err := ctx.Client("")
	require.NoError(t, err)
	require.NotNil(t, child)

	start := time.Now()
	_, err = child.Post(srv.URL, map[string]string{"k": "v"}, nil)
	require.Error(t, err, "a Client-derived context must be bounded by the same timeout")
	assert.Contains(t, err.Error(), "timed out after 200ms")
	assert.Less(t, time.Since(start), time.Second)
}

func TestTokenHTTPContext_CADerivedContextAttachesToken(t *testing.T) {
	t.Setenv("KYVERNO_SCOPED_TOKEN_PATH", writeTokenFile(t, "unit-token\n"))

	var gotAuth string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	caBundle := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	require.NotEmpty(t, caBundle)

	ctx := newTokenContext(t)
	child, err := ctx.Client(string(caBundle))
	require.NoError(t, err)
	require.NotNil(t, child)

	_, err = child.Post(srv.URL, map[string]string{"k": "v"}, nil)
	require.NoError(t, err, "a context built with a trusted CA bundle must reach the TLS server")
	assert.Equal(t, "Bearer unit-token", gotAuth)
}

func TestTokenHTTPContext_InvalidCABundleReturnsError(t *testing.T) {
	ctx := newTokenContext(t)

	child, err := ctx.Client("this is not a PEM bundle")
	require.Error(t, err)
	assert.Nil(t, child, "a context must not be returned when the CA bundle cannot be parsed")
}

func TestTokenHTTPContext_EmptyTokenFileSendsNoHeader(t *testing.T) {
	for name, contents := range map[string]string{
		"empty file":      "",
		"whitespace only": " \n\t ",
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("KYVERNO_SCOPED_TOKEN_PATH", writeTokenFile(t, contents))
			srv, gotAuth := captureAuthServer(t)
			ctx := newTokenContext(t)

			headers := map[string]string{"X-Test": "1"}
			assert.Equal(t, headers, withScopedToken(headers), "an unusable token must leave the headers untouched")

			_, err := ctx.Get(srv.URL, headers)
			require.NoError(t, err)
			assert.Equal(t, "", *gotAuth, "no Authorization header may be sent for an empty token file")
			assert.NotContains(t, headers, "Authorization")
		})
	}
}
