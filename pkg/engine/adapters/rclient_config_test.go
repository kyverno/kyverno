package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/sdk/extensions/registryclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rawManifest struct{ raw []byte }

func (r rawManifest) RawManifest() ([]byte, error) { return r.raw, nil }
func (r rawManifest) MediaType() (types.MediaType, error) {
	return types.DockerManifestSchema2, nil
}

// pushImageWithConfig publishes an image whose config blob is configSize bytes and returns its tagged ref.
func pushImageWithConfig(t *testing.T, configSize int) string {
	t.Helper()
	srv := httptest.NewServer(registry.New())
	t.Cleanup(srv.Close)

	repo, err := name.NewRepository(strings.TrimPrefix(srv.URL, "http://") + "/test/cfg")
	require.NoError(t, err)

	cfg, err := json.Marshal(map[string]any{
		"architecture": "amd64",
		"os":           "linux",
		"rootfs":       map[string]any{"type": "layers", "diff_ids": []string{}},
		"padding":      string(bytes.Repeat([]byte("A"), configSize)),
	})
	require.NoError(t, err)

	cfgLayer := static.NewLayer(cfg, types.DockerConfigJSON)
	require.NoError(t, remote.WriteLayer(repo, cfgLayer))
	cfgDigest, err := cfgLayer.Digest()
	require.NoError(t, err)
	cfgSize, err := cfgLayer.Size()
	require.NoError(t, err)

	raw, err := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     string(types.DockerManifestSchema2),
		"config": map[string]any{
			"mediaType": string(types.DockerConfigJSON),
			"digest":    cfgDigest.String(),
			"size":      cfgSize,
		},
		"layers": []any{},
	})
	require.NoError(t, err)

	tag := repo.Tag("latest")
	require.NoError(t, remote.Put(tag, rawManifest{raw: raw}))
	return tag.Name()
}

func TestForRef_RejectsOversizedConfig(t *testing.T) {
	ref := pushImageWithConfig(t, int(maxConfigSize)+1)
	adapter := RegistryClient(registryclient.New())
	require.NotNil(t, adapter)

	_, err := adapter.ForRef(context.Background(), ref)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config size")
	assert.Contains(t, err.Error(), "exceeds")
}

func TestForRef_AcceptsNormalConfig(t *testing.T) {
	ref := pushImageWithConfig(t, 1024)
	adapter := RegistryClient(registryclient.New())
	require.NotNil(t, adapter)

	result, err := adapter.ForRef(context.Background(), ref)
	require.NoError(t, err)
	require.NotNil(t, result)
}
