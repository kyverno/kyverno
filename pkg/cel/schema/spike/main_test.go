package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/pkg/cel/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const policy = `object.request.capability_type != 'command' || object.request.command != 'kubectl' || !derived.isMutatingKubectl`

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	dir := filepath.Join("..", "testdata", "nono")
	sj, err := os.ReadFile(filepath.Join(dir, "schema.json"))
	require.NoError(t, err)
	cj, err := os.ReadFile(filepath.Join(dir, "computed.json"))
	require.NoError(t, err)
	var computed []schema.Computed
	require.NoError(t, json.Unmarshal(cj, &computed))
	s, err := schema.New(sj, computed)
	require.NoError(t, err)
	srv := httptest.NewServer(handler(s, policy))
	t.Cleanup(srv.Close)
	return srv
}

func TestApprove(t *testing.T) {
	t.Parallel()
	srv := newServer(t)
	tests := map[string]string{
		"kubectl-get.json":                "granted",
		"kubectl-delete-ns.json":          "denied",
		"endpoint.json":                   "granted",
		"invalid-missing-request-id.json": "denied",
		"invalid-pid-type.json":           "denied",
	}
	for fixture, want := range tests {
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()
			body, err := os.ReadFile(filepath.Join("..", "testdata", "nono", fixture))
			require.NoError(t, err)
			resp, err := http.Post(srv.URL+"/approve", "application/json", strings.NewReader(string(body)))
			require.NoError(t, err)
			defer resp.Body.Close()
			var got response
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
			assert.Equal(t, want, got.Decision)
		})
	}
}

func TestApproveFailsClosed(t *testing.T) {
	t.Parallel()
	srv := newServer(t)
	resp, err := http.Post(srv.URL+"/approve", "application/json", strings.NewReader("not json"))
	require.NoError(t, err)
	defer resp.Body.Close()
	var got response
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.Equal(t, "denied", got.Decision)

	resp2, err := http.Get(srv.URL + "/approve")
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, resp2.StatusCode)
}
