package apicall

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/toggle"
	"gotest.tools/v3/assert"
)

func withEmptyEgressBlocklist(t *testing.T) {
	t.Helper()
	assert.NilError(t, toggle.HTTPBlocklist.Parse(""))
	t.Cleanup(toggle.HTTPBlocklist.Reset)
}

func testExecutor() *executor {
	return NewExecutor(logr.Discard(), "test", &mockClient{}, apiConfig)
}

func getServiceCall(url string) *kyvernov1.APICall {
	return &kyvernov1.APICall{Method: "GET", Service: &kyvernov1.ServiceCall{URL: url}}
}

func Test_ExecuteServiceCall_BlocksLoopbackByDefault(t *testing.T) {
	toggle.HTTPBlocklist.Reset()

	var gotHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := testExecutor().Execute(context.Background(), getServiceCall(srv.URL))
	assert.ErrorContains(t, err, "blocked")
	assert.Assert(t, !gotHit)
}

func Test_ExecuteServiceCall_BlocksMetadataHostByDefault(t *testing.T) {
	toggle.HTTPBlocklist.Reset()

	_, err := testExecutor().Execute(context.Background(), getServiceCall("http://169.254.169.254/latest/meta-data/"))
	assert.ErrorContains(t, err, "blocked")
}

func Test_ExecuteServiceCall_AllowsPermittedHost(t *testing.T) {
	withEmptyEgressBlocklist(t)

	tokenPath := filepath.Join(t.TempDir(), "token")
	assert.NilError(t, os.WriteFile(tokenPath, []byte("scoped-token"), 0o600))
	withScopedTokenPath(t, tokenPath)

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	body, err := testExecutor().Execute(context.Background(), getServiceCall(srv.URL))
	assert.NilError(t, err)
	assert.Equal(t, string(body), `{"ok":true}`)
	assert.Equal(t, gotAuth, "Bearer scoped-token")
}

func Test_ExecuteServiceCall_AllowlistRejectsOtherHosts(t *testing.T) {
	withEmptyEgressBlocklist(t)
	assert.NilError(t, toggle.HTTPAllowlist.Parse("https://api.example.com"))
	t.Cleanup(toggle.HTTPAllowlist.Reset)

	_, err := testExecutor().Execute(context.Background(), getServiceCall("https://not-allowed.example.org/data"))
	assert.ErrorContains(t, err, "not permitted")
}

func Test_egressPolicy_dialContext_BlocksResolvedIP(t *testing.T) {
	p, err := newEgressPolicy([]string{"127.0.0.0/8"}, nil)
	assert.NilError(t, err)

	_, err = p.dialContext()(context.Background(), "tcp", net.JoinHostPort("localhost", "80"))
	assert.ErrorContains(t, err, "blocked")
}
