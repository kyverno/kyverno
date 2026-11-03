// Package ocitest holds registry test support shared across the oci subcommand packages'
// own test files. It is test support only; nothing here is imported by non-test code.
package ocitest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/stretchr/testify/require"
)

// NewRegistry starts an in-process, anonymous-access OCI registry and returns its host:port,
// closing the server when t's test ends. It replaces the inline
// httptest.NewServer(registry.New()) copy duplicated across pull, validate, inspect, and resolve's
// own test files.
func NewRegistry(t *testing.T) (host string) {
	t.Helper()
	srv := httptest.NewServer(registry.New())
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return u.Host
}

// NewRecordingRegistry works like NewRegistry, but also returns a function that lists every
// request path the registry has received since the last reset, in order, and a function to reset
// that log. It lets a test push a fixture (whose own blob-existence checks and uploads would
// otherwise pollute the log), reset, and then assert that a specific read only touched the blobs
// it should have.
func NewRecordingRegistry(t *testing.T) (host string, requests func() []string, reset func()) {
	t.Helper()
	var mu sync.Mutex
	var paths []string

	handler := registry.New()
	recorder := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		handler.ServeHTTP(w, r)
	})

	srv := httptest.NewServer(recorder)
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return u.Host,
		func() []string {
			mu.Lock()
			defer mu.Unlock()
			out := make([]string, len(paths))
			copy(out, paths)
			return out
		},
		func() {
			mu.Lock()
			defer mu.Unlock()
			paths = nil
		}
}
