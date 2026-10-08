package apicall

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func tokenFile(t *testing.T, contents string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "token")
	assert.NilError(t, os.WriteFile(p, []byte(contents), 0o600))
	return p
}

// An explicitly empty Authorization header is an opt-out and must suppress token
// injection; Get cannot tell it apart from an absent header, so presence is tested.
func Test_addHTTPHeaders_ExplicitEmptyAuthorizationSuppressesToken(t *testing.T) {
	withScopedTokenPath(t, tokenFile(t, "leaked-token"))

	req, err := http.NewRequest(http.MethodGet, "http://example.invalid", nil)
	assert.NilError(t, err)
	req.Header.Add("Authorization", "")

	a := &executor{}
	assert.NilError(t, a.addHTTPHeaders(req, nil))

	got := req.Header.Values("Authorization")
	assert.Equal(t, len(got), 1, "an explicit empty Authorization must not be joined by an injected token")
	assert.Equal(t, got[0], "")
}

func Test_addHTTPHeaders_AbsentAuthorizationGetsToken(t *testing.T) {
	withScopedTokenPath(t, tokenFile(t, "scoped-token"))

	req, err := http.NewRequest(http.MethodGet, "http://example.invalid", nil)
	assert.NilError(t, err)

	a := &executor{}
	assert.NilError(t, a.addHTTPHeaders(req, nil))

	got := req.Header.Values("Authorization")
	assert.Equal(t, len(got), 1)
	assert.Equal(t, got[0], "Bearer scoped-token")
}

// The same opt-out must hold on the scoped token client used by CEL http calls.
func Test_scopedTokenClient_Do_ExplicitEmptyAuthorizationSuppressesToken(t *testing.T) {
	withScopedTokenPath(t, tokenFile(t, "leaked-token"))

	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Values("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	assert.NilError(t, err)
	req.Header.Add("Authorization", "")

	resp, err := NewScopedTokenClient().Do(req)
	assert.NilError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, len(got), 1, "an explicit empty Authorization must not be joined by an injected token")
	assert.Equal(t, got[0], "")
}
