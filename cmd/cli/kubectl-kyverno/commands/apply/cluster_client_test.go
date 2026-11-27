package apply

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func kubeconfigForHost(t *testing.T, host string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kubeconfig")
	content := "apiVersion: v1\nkind: Config\nclusters:\n- name: test\n  cluster:\n    server: " + host + "\ncontexts:\n- name: test\n  context:\n    cluster: test\ncurrent-context: test\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestInitStoreAndClusterClient_UnreachableCluster(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())

	config := ApplyCommandConfig{
		Cluster:    true,
		KubeConfig: kubeconfigForHost(t, "http://"+addr),
	}
	_, err = config.initStoreAndClusterClient(&store.Store{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to cluster")
}

func TestInitStoreAndClusterClient_ReachableCluster(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kind":"APIGroupList","apiVersion":"v1","groups":[]}`))
	}))
	defer server.Close()

	config := ApplyCommandConfig{
		Cluster:    true,
		KubeConfig: kubeconfigForHost(t, server.URL),
	}
	dClient, err := config.initStoreAndClusterClient(&store.Store{})
	require.NoError(t, err)
	assert.NotNil(t, dClient)
}
