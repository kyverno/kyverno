// Package ocitest holds registry test support shared across the oci subcommand packages'
// own test files. It is test support only; nothing here is imported by non-test code.
package ocitest

import (
	"net/http/httptest"
	"net/url"
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
