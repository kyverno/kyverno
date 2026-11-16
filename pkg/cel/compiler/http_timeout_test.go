package compiler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	celhttp "github.com/kyverno/sdk/extensions/cel/libs/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // SetHTTPCallTimeout is process-global
func TestHTTPCallTimeout(t *testing.T) {
	t.Cleanup(func() { SetHTTPCallTimeout(0) })
	release := make(chan struct{})
	mux := http.NewServeMux()
	// sends the headers, then stalls the body until the test ends
	mux.HandleFunc("/stall-body", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"partial":`))
		w.(http.Flusher).Flush()
		<-release
	})
	mux.HandleFunc("/ok", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	// cleanups run last-in first-out: release the stalled handlers before closing the server
	t.Cleanup(func() { close(release) })

	// a CIDR blocklist that does not cover the test server builds the guarded client kyverno uses
	inner, err := celhttp.NewHTTPWithBlocklist([]string{"192.0.2.0/24"}, nil)
	require.NoError(t, err)
	ctx := boundedHTTPContext{inner: inner}
	withCA, err := ctx.Client("")
	require.NoError(t, err)

	SetHTTPCallTimeout(200 * time.Millisecond)
	for name, call := range map[string]func() (any, error){
		"get":         func() (any, error) { return ctx.Get(srv.URL+"/stall-body", nil) },
		"post":        func() (any, error) { return ctx.Post(srv.URL+"/stall-body", map[string]any{"a": 1}, nil) },
		"client get":  func() (any, error) { return withCA.Get(srv.URL+"/stall-body", nil) },
		"client post": func() (any, error) { return withCA.Post(srv.URL+"/stall-body", nil, nil) },
	} {
		start := time.Now()
		_, err := call()
		require.ErrorContains(t, err, "did not complete within 200ms", name)
		assert.Less(t, time.Since(start), 5*time.Second, name)
	}

	value, err := ctx.Get(srv.URL+"/ok", nil)
	require.NoError(t, err)
	assert.Equal(t, true, value.(map[string]any)["ok"])

	SetHTTPCallTimeout(0)
	value, err = ctx.Get(srv.URL+"/ok", nil)
	require.NoError(t, err, "zero disables the bound")
	assert.Equal(t, true, value.(map[string]any)["ok"])
}
