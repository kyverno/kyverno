package internal

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/stretchr/testify/require"
)

func TestLoadTUFRootGuardsOperatorURL(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte("root"))
	}))
	defer server.Close()
	_, err := loadTUFRoot(context.Background(), server.URL+"/root.json", "")
	require.ErrorIs(t, err, egress.ErrAddressBlocked)
	require.Zero(t, requests.Load())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = loadTUFRoot(ctx, "https://registry.example/root.json", "")
	require.ErrorIs(t, err, context.Canceled)
}

func TestLoadTUFRootPreservesLocalAndInlineConfiguration(t *testing.T) {
	data, err := loadTUFRoot(context.Background(), "", "")
	require.NoError(t, err)
	require.Nil(t, data)
	raw := base64.StdEncoding.EncodeToString([]byte("inline root"))
	data, err = loadTUFRoot(context.Background(), "", raw)
	require.NoError(t, err)
	require.Equal(t, []byte("inline root"), data)
	file := filepath.Join(t.TempDir(), "root.json")
	require.NoError(t, os.WriteFile(file, []byte("file root"), 0o600))
	data, err = loadTUFRoot(context.Background(), file, raw)
	require.NoError(t, err)
	require.Equal(t, []byte("file root"), data)
	t.Setenv("KYVERNO_TEST_TUF_ROOT", "environment root")
	data, err = loadTUFRoot(context.Background(), "env://KYVERNO_TEST_TUF_ROOT", "")
	require.NoError(t, err)
	require.Equal(t, []byte("environment root"), data)
	_, err = loadTUFRoot(context.Background(), "", "invalid base64")
	require.ErrorContains(t, err, "decoding alternate TUF root")
	_, err = loadTUFRoot(context.Background(), file+".missing", raw)
	require.ErrorIs(t, err, os.ErrNotExist)
}
