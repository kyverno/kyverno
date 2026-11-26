// Package imagetest provides an in-process OCI 1.1 registry and ephemeral
// virtual Sigstore helpers for offline image verification tests.
//
// This package intentionally omits a //go:build integration tag so unit tests
// under pkg/image/verifiers/ivpol/cosign can share the same helpers alongside
// test/integration/ivpol without build-tag friction or import cycles.
package imagetest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/require"
)

const (
	// BundleMediaType is the OCI media type for Sigstore v0.3 bundles.
	BundleMediaType = "application/vnd.dev.sigstore.bundle.v0.3+json"
	// emptyConfigMediaType is the OCI empty config media type used for referrer manifests.
	emptyConfigMediaType = "application/vnd.oci.empty.v1+json"
)

type rawTaggable struct {
	raw []byte
	mt  types.MediaType
}

func (t rawTaggable) RawManifest() ([]byte, error)        { return t.raw, nil }
func (t rawTaggable) MediaType() (types.MediaType, error) { return t.mt, nil }

// Registry wraps an in-process OCI 1.1 registry with referrers support and a blob GET counter.
type Registry struct {
	Repo     name.Repository
	BlobGets *atomic.Int64
}

// NewRegistry starts an in-process OCI registry with referrers support on loopback HTTP.
// The server is automatically closed when t finishes.
func NewRegistry(t *testing.T) *Registry {
	t.Helper()
	var blobGets atomic.Int64
	inner := registry.New(registry.WithReferrersSupport(true))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/blobs/sha256:") {
			blobGets.Add(1)
		}
		inner.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	repo, err := name.NewRepository(strings.TrimPrefix(srv.URL, "http://") + "/test/img")
	require.NoError(t, err)
	return &Registry{
		Repo:     repo,
		BlobGets: &blobGets,
	}
}

// PushSubject writes a small random image under tag in repo and returns its OCI descriptor.
func PushSubject(t *testing.T, repo name.Repository, tag string) v1.Descriptor {
	t.Helper()
	img, err := random.Image(256, 1)
	require.NoError(t, err)
	require.NoError(t, remote.Write(repo.Tag(tag), img))
	d, err := img.Digest()
	require.NoError(t, err)
	sz, err := img.Size()
	require.NoError(t, err)
	mt, err := img.MediaType()
	require.NoError(t, err)
	return v1.Descriptor{MediaType: mt, Digest: d, Size: sz}
}

// PushSubject writes a small random image under tag in this registry and returns its OCI descriptor.
func (r *Registry) PushSubject(t *testing.T, tag string) v1.Descriptor {
	t.Helper()
	return PushSubject(t, r.Repo, tag)
}

// PushReferrer attaches a single-layer OCI 1.1 referrer manifest to subject in repo.
func PushReferrer(t *testing.T, repo name.Repository, subject v1.Descriptor, layerData []byte, layerMediaType, artifactType string) {
	t.Helper()
	layer := static.NewLayer(layerData, types.MediaType(layerMediaType))
	require.NoError(t, remote.WriteLayer(repo, layer))
	layerDigest, err := layer.Digest()
	require.NoError(t, err)

	cfg := static.NewLayer([]byte("{}"), types.MediaType(emptyConfigMediaType))
	require.NoError(t, remote.WriteLayer(repo, cfg))
	cfgDigest, err := cfg.Digest()
	require.NoError(t, err)

	m := v1.Manifest{
		SchemaVersion: 2,
		MediaType:     types.OCIManifestSchema1,
		ArtifactType:  artifactType,
		Config:        v1.Descriptor{MediaType: types.MediaType(emptyConfigMediaType), Digest: cfgDigest, Size: 2},
		Layers: []v1.Descriptor{{
			MediaType: types.MediaType(layerMediaType),
			Digest:    layerDigest,
			Size:      int64(len(layerData)),
		}},
		Subject: &subject,
	}
	raw, err := json.Marshal(m)
	require.NoError(t, err)
	h, _, err := v1.SHA256(bytes.NewReader(raw))
	require.NoError(t, err)
	require.NoError(t, remote.Put(repo.Digest(h.String()), rawTaggable{raw: raw, mt: types.OCIManifestSchema1}))
}

// PushReferrer attaches a single-layer OCI 1.1 referrer manifest to subject in this registry.
func (r *Registry) PushReferrer(t *testing.T, subject v1.Descriptor, layerData []byte, layerMediaType, artifactType string) {
	t.Helper()
	PushReferrer(t, r.Repo, subject, layerData, layerMediaType, artifactType)
}

// PinnedRef returns the tag@digest reference string for subject in this registry,
// or a digest-only reference when tag is empty.
func (r *Registry) PinnedRef(tag string, subject v1.Descriptor) string {
	if tag == "" {
		return r.Repo.Digest(subject.Digest.String()).String()
	}
	return r.Repo.Tag(tag).String() + "@" + subject.Digest.String()
}
