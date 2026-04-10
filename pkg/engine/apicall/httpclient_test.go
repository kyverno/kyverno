package apicall

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"gotest.tools/assert"
)

func Test_NewScopedTokenClient_Defaults(t *testing.T) {
	withScopedTokenClientTimeout(t, defaultScopedTokenClientTimeout)
	client := NewScopedTokenClient()

	assert.Equal(t, client.inner.Timeout, defaultScopedTokenClientTimeout)
	_, ok := client.inner.Transport.(*otelhttp.Transport)
	assert.Check(t, ok)
}

func Test_NewScopedTokenClient_UsesConfiguredTimeout(t *testing.T) {
	withScopedTokenClientTimeout(t, 7*time.Second)
	client := NewScopedTokenClient()

	assert.Equal(t, client.inner.Timeout, 7*time.Second)
}

func withScopedTokenClientTimeout(t *testing.T, timeout time.Duration) {
	t.Helper()
	old := scopedTokenClientTimeout
	SetScopedTokenClientTimeout(timeout)
	t.Cleanup(func() {
		SetScopedTokenClientTimeout(old)
	})
}

func withScopedTokenPath(t *testing.T, path string) {
	t.Helper()
	old := scopedTokenPath
	scopedTokenPath = path
	t.Cleanup(func() {
		scopedTokenPath = old
	})
}

func Test_scopedTokenClient_Do_SetsAuthorizationWhenAbsent(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "token")
	assert.NilError(t, os.WriteFile(tokenPath, []byte("  test-token\n"), 0o600))
	withScopedTokenPath(t, tokenPath)

	var gotAuth string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	client := NewScopedTokenClient()
	client.inner = s.Client()

	req, err := http.NewRequest(http.MethodGet, s.URL, nil)
	assert.NilError(t, err)

	resp, err := client.Do(req)
	assert.NilError(t, err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	assert.Equal(t, gotAuth, "Bearer test-token")
}

func Test_scopedTokenClient_Do_DoesNotOverrideAuthorizationWhenPresent(t *testing.T) {
	missingTokenPath := filepath.Join(t.TempDir(), "missing-token")
	withScopedTokenPath(t, missingTokenPath)

	var gotAuth string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	client := NewScopedTokenClient()
	client.inner = s.Client()

	req, err := http.NewRequest(http.MethodGet, s.URL, nil)
	assert.NilError(t, err)
	req.Header.Set("Authorization", "Bearer provided-token")

	resp, err := client.Do(req)
	assert.NilError(t, err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	assert.Equal(t, gotAuth, "Bearer provided-token")
}

func Test_scopedTokenClient_Do_TokenMissingDoesNotFailRequest(t *testing.T) {
	missingTokenPath := filepath.Join(t.TempDir(), "missing-token")
	withScopedTokenPath(t, missingTokenPath)

	var gotAuth string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	client := NewScopedTokenClient()
	client.inner = s.Client()

	req, err := http.NewRequest(http.MethodGet, s.URL, nil)
	assert.NilError(t, err)

	resp, err := client.Do(req)
	assert.NilError(t, err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	assert.Equal(t, gotAuth, "")
}
