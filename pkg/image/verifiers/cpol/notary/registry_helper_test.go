package notary

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
)

// rawManifest is a minimal remote.Taggable for pushing hand-built manifests.
type rawManifest struct {
	raw []byte
	mt  types.MediaType
}

func (r rawManifest) RawManifest() ([]byte, error)        { return r.raw, nil }
func (r rawManifest) MediaType() (types.MediaType, error) { return r.mt, nil }

// newTestRegistry starts an in-process OCI registry and returns a repository targeting it.
func newTestRegistry(t *testing.T) name.Repository {
	t.Helper()
	srv := httptest.NewServer(registry.New())
	t.Cleanup(srv.Close)
	repo, err := name.NewRepository(strings.TrimPrefix(srv.URL, "http://") + "/test/img")
	assert.NilError(t, err)
	return repo
}

// pushBlob writes a layer built from data to the repository and returns its digest and size.
func pushBlob(t *testing.T, repo name.Repository, data []byte, mt string) (name.Digest, int64) {
	t.Helper()
	layer := static.NewLayer(data, types.MediaType(mt))
	assert.NilError(t, remote.WriteLayer(repo, layer))
	h, err := layer.Digest()
	assert.NilError(t, err)
	return repo.Digest(h.String()), int64(len(data))
}

// pushManifest marshals m and pushes it to the repository, returning its digest reference.
func pushManifest(t *testing.T, repo name.Repository, m ocispec.Manifest) name.Digest {
	t.Helper()
	raw, err := json.Marshal(m)
	assert.NilError(t, err)
	h, _, err := v1.SHA256(bytes.NewReader(raw))
	assert.NilError(t, err)
	ref := repo.Digest(h.String())
	assert.NilError(t, remote.Put(ref, rawManifest{raw: raw, mt: types.MediaType(m.MediaType)}))
	return ref
}

// gzipBytes returns n zero bytes gzip-compressed, for building highly-compressible fixtures.
func gzipBytes(t *testing.T, n int) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, err := gw.Write(make([]byte, n))
	assert.NilError(t, err)
	assert.NilError(t, gw.Close())
	return buf.Bytes()
}
