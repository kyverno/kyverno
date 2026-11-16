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

//nolint:paralleltest // SetHTTPCallTimeout and the abandoned-call cap are process-global
func TestAbandonedHTTPCallsAreCapped(t *testing.T) {
	previousMax := maxAbandonedHTTPCalls
	t.Cleanup(func() {
		SetHTTPCallTimeout(0)
		maxAbandonedHTTPCalls = previousMax
	})
	// calls abandoned by other tests must have drained before counting
	require.Eventually(t, func() bool { return abandonedHTTPCalls.Load() == 0 }, 5*time.Second, 10*time.Millisecond)
	maxAbandonedHTTPCalls = 2
	release := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/stall", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-release
	})
	mux.HandleFunc("/ok", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	released := false
	t.Cleanup(func() {
		if !released {
			close(release)
		}
	})
	ctx := boundedHTTPContext{inner: celhttp.NewHTTP()}
	SetHTTPCallTimeout(300 * time.Millisecond)

	for i := 0; i < 2; i++ {
		_, err := ctx.Get(srv.URL+"/stall", nil)
		require.ErrorContains(t, err, "did not complete within")
	}
	assert.EqualValues(t, 2, abandonedHTTPCalls.Load())

	start := time.Now()
	_, err := ctx.Get(srv.URL+"/ok", nil)
	require.ErrorContains(t, err, "http call refused: 2 earlier calls timed out")
	assert.Less(t, time.Since(start), 250*time.Millisecond, "refused at once, without waiting for the timeout")

	// once the endpoint lets go, the abandoned calls finish and calls are accepted again
	close(release)
	released = true
	require.Eventually(t, func() bool { return abandonedHTTPCalls.Load() == 0 }, 5*time.Second, 10*time.Millisecond)
	value, err := ctx.Get(srv.URL+"/ok", nil)
	require.NoError(t, err)
	assert.Equal(t, true, value.(map[string]any)["ok"])
}
