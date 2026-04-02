package apicall

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/assert"
)

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

func Test_scopedTokenClient_Do_ReturnsErrorWhenTokenMissing(t *testing.T) {
	missingTokenPath := filepath.Join(t.TempDir(), "missing-token")
	withScopedTokenPath(t, missingTokenPath)

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	client := NewScopedTokenClient()
	client.inner = s.Client()

	req, err := http.NewRequest(http.MethodGet, s.URL, nil)
	assert.NilError(t, err)

	_, err = client.Do(req)
	assert.Check(t, err != nil)
	assert.ErrorContains(t, err, "failed to read required scoped APICall token")
	assert.Check(t, strings.Contains(err.Error(), missingTokenPath))
}
