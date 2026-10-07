package cosign

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
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
	"gotest.tools/v3/assert"
)

// rawManifest is a manifest pushed as-is, so tests control fields such as artifactType.
type rawManifest struct {
	body      []byte
	mediaType types.MediaType
}

func (m rawManifest) RawManifest() ([]byte, error)        { return m.body, nil }
func (m rawManifest) MediaType() (types.MediaType, error) { return m.mediaType, nil }

type testReferrer struct {
	// descriptorArtifactType is the artifact type in the fallback tag index entry
	descriptorArtifactType string
	manifestArtifactType   string
	layerMediaType         string
	layerData              []byte
	// skipLayerUpload leaves the layer blob out of the registry, so reading it fails
	skipLayerUpload bool
	// descriptorSize overrides the manifest size in the fallback tag index entry
	descriptorSize int64
	// manifestPadding adds an annotation of this many bytes to the manifest
	manifestPadding int
}

// pushReferrer pushes a single-layer manifest and returns its descriptor for the fallback index.
func pushReferrer(t *testing.T, repo name.Repository, r testReferrer) v1.Descriptor {
	t.Helper()
	config := static.NewLayer([]byte("{}"), types.MediaType(ociEmptyArtifactType))
	layer := static.NewLayer(r.layerData, types.MediaType(r.layerMediaType))
	descriptor := func(l v1.Layer, upload bool) map[string]any {
		digest, err := l.Digest()
		assert.NilError(t, err)
		size, err := l.Size()
		assert.NilError(t, err)
		mediaType, err := l.MediaType()
		assert.NilError(t, err)
		if upload {
			assert.NilError(t, remote.WriteLayer(repo, l))
		}
		return map[string]any{"mediaType": mediaType, "digest": digest.String(), "size": size}
	}
	manifest := map[string]any{
		"schemaVersion": 2,
		"mediaType":     types.OCIManifestSchema1,
		"config":        descriptor(config, true),
		"layers":        []any{descriptor(layer, !r.skipLayerUpload)},
	}
	if r.manifestArtifactType != "" {
		manifest["artifactType"] = r.manifestArtifactType
	}
	if r.manifestPadding > 0 {
		manifest["annotations"] = map[string]string{"padding": strings.Repeat("x", r.manifestPadding)}
	}
	body, err := json.Marshal(manifest)
	assert.NilError(t, err)
	digest, size, err := v1.SHA256(bytes.NewReader(body))
	assert.NilError(t, err)
	assert.NilError(t, remote.Put(repo.Digest(digest.String()), rawManifest{body: body, mediaType: types.OCIManifestSchema1}))
	if r.descriptorSize != 0 {
		size = r.descriptorSize
	}
	return v1.Descriptor{
		MediaType:    types.OCIManifestSchema1,
		Digest:       digest,
		Size:         size,
		ArtifactType: r.descriptorArtifactType,
	}
}

// setupFallbackRegistry serves an image whose referrers are only listed in the fallback tag index,
// as on registries without the OCI 1.1 referrers API.
func setupFallbackRegistry(t *testing.T, referrers []testReferrer) name.Reference {
	t.Helper()
	server := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(server.Close)
	repo, err := name.NewRepository(strings.TrimPrefix(server.URL, "http://") + "/test/app")
	assert.NilError(t, err)

	img, err := random.Image(64, 1)
	assert.NilError(t, err)
	ref := repo.Tag("latest")
	assert.NilError(t, remote.Write(ref, img))
	imgDigest, err := img.Digest()
	assert.NilError(t, err)

	manifests := make([]v1.Descriptor, 0, len(referrers))
	for _, r := range referrers {
		manifests = append(manifests, pushReferrer(t, repo, r))
	}
	index, err := json.Marshal(v1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     types.OCIImageIndex,
		Manifests:     manifests,
	})
	assert.NilError(t, err)
	fallbackTag := repo.Tag(fmt.Sprintf("%s-%s", imgDigest.Algorithm, imgDigest.Hex))
	assert.NilError(t, remote.Put(fallbackTag, rawManifest{body: index, mediaType: types.OCIImageIndex}))
	return ref
}

func TestFetchBundlesFallbackTag(t *testing.T) {
	t.Parallel()
	bundleJSON, err := os.ReadFile("testdata/bundle.json")
	assert.NilError(t, err)
	bundleMediaType := "application/vnd.dev.sigstore.bundle.v0.3+json"
	tooLarge := append(bytes.Clone(bundleJSON), bytes.Repeat([]byte(" "), int(maxProbeLayerSize))...)

	tests := []struct {
		name     string
		referrer testReferrer
		want     int
	}{{
		name: "empty descriptor type resolved by manifest artifact type",
		referrer: testReferrer{
			manifestArtifactType: bundleMediaType,
			layerMediaType:       "application/json",
			layerData:            bundleJSON,
		},
		want: 1,
	}, {
		name: "OCI empty descriptor type resolved by layer media type",
		referrer: testReferrer{
			descriptorArtifactType: ociEmptyArtifactType,
			layerMediaType:         bundleMediaType,
			layerData:              bundleJSON,
		},
		want: 1,
	}, {
		name: "untyped layer resolved by parsing the bundle",
		referrer: testReferrer{
			layerMediaType: "application/json",
			layerData:      bundleJSON,
		},
		want: 1,
	}, {
		name: "untyped layer that is not a bundle is skipped",
		referrer: testReferrer{
			layerMediaType: "application/json",
			layerData:      []byte(`{"hello":"world"}`),
		},
	}, {
		name: "known non-bundle layer type is skipped without probing",
		referrer: testReferrer{
			layerMediaType: "application/vnd.oci.image.layer.v1.tar",
			layerData:      bundleJSON,
		},
	}, {
		name: "untyped layer over the probe size limit is skipped",
		referrer: testReferrer{
			layerMediaType: "application/json",
			layerData:      tooLarge,
		},
	}, {
		name: "untyped manifest over the probe size limit is skipped without fetching it",
		referrer: testReferrer{
			manifestArtifactType: bundleMediaType,
			layerMediaType:       bundleMediaType,
			layerData:            bundleJSON,
			descriptorSize:       maxProbeManifestSize + 1,
		},
	}, {
		name: "manifest larger than its understated index size is skipped without downloading it",
		referrer: testReferrer{
			manifestArtifactType: bundleMediaType,
			layerMediaType:       bundleMediaType,
			layerData:            bundleJSON,
			descriptorSize:       100,
			manifestPadding:      int(maxProbeManifestSize),
		},
	}, {
		name: "layer typed as a bundle that does not parse as one is skipped",
		referrer: testReferrer{
			manifestArtifactType: bundleMediaType,
			layerMediaType:       bundleMediaType,
			layerData:            []byte(`{"hello":"world"}`),
		},
	}, {
		name: "typed descriptor of another artifact is skipped",
		referrer: testReferrer{
			descriptorArtifactType: "application/vnd.example.sbom",
			manifestArtifactType:   bundleMediaType,
			layerMediaType:         bundleMediaType,
			layerData:              bundleJSON,
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ref := setupFallbackRegistry(t, []testReferrer{tt.referrer})
			bundles, desc, err := fetchBundles(context.Background(), ref, attestationlimit, "", nil, nil)
			assert.NilError(t, err)
			assert.Assert(t, desc != nil)
			assert.Equal(t, len(bundles), tt.want)
		})
	}
}

func TestFetchBundlesFallbackProbeBudget(t *testing.T) {
	t.Parallel()
	bundleJSON, err := os.ReadFile("testdata/bundle.json")
	assert.NilError(t, err)
	// each untyped layer is just under the per-layer limit, so the shared budget runs out first
	padded := append(bytes.Clone(bundleJSON), bytes.Repeat([]byte(" "), int(maxProbeLayerSize)-len(bundleJSON)-1)...)
	count := int(maxProbeTotalSize/maxProbeLayerSize) + 2
	referrers := make([]testReferrer, count)
	for i := range referrers {
		referrers[i] = testReferrer{layerMediaType: "application/json", layerData: padded}
	}
	ref := setupFallbackRegistry(t, referrers)
	bundles, _, err := fetchBundles(context.Background(), ref, attestationlimit, "", nil, nil)
	assert.NilError(t, err)
	// manifests are charged to the budget too, so the last layer that would just fit is skipped
	fit := int(maxProbeTotalSize / int64(len(padded)))
	assert.Assert(t, len(bundles) >= fit-1 && len(bundles) <= fit, "got %d bundles, want %d or %d", len(bundles), fit-1, fit)
}

func TestFetchBundlesFallbackTypedReferrersShareBudget(t *testing.T) {
	t.Parallel()
	bundleJSON, err := os.ReadFile("testdata/bundle.json")
	assert.NilError(t, err)
	// referrers recognized by their media type are read within the shared probe budget too
	padded := append(bytes.Clone(bundleJSON), bytes.Repeat([]byte(" "), int(maxProbeLayerSize)-len(bundleJSON)-1)...)
	count := int(maxProbeTotalSize/maxProbeLayerSize) + 2
	referrers := make([]testReferrer, count)
	for i := range referrers {
		referrers[i] = testReferrer{
			manifestArtifactType: "application/vnd.dev.sigstore.bundle.v0.3+json",
			layerMediaType:       "application/json",
			layerData:            padded,
		}
	}
	ref := setupFallbackRegistry(t, referrers)
	bundles, _, err := fetchBundles(context.Background(), ref, attestationlimit, "", nil, nil)
	assert.NilError(t, err)
	fit := int(maxProbeTotalSize / int64(len(padded)))
	assert.Assert(t, len(bundles) >= fit-1 && len(bundles) <= fit, "got %d bundles, want %d or %d", len(bundles), fit-1, fit)
}

func TestReadLayerLimits(t *testing.T) {
	t.Parallel()
	data := []byte("0123456789")
	got, err := readLayer(static.NewLayer(data, types.MediaType("application/json")), int64(len(data)))
	assert.NilError(t, err)
	assert.DeepEqual(t, got, data)

	// the size check rejects the layer before reading
	_, err = readLayer(static.NewLayer(data, types.MediaType("application/json")), int64(len(data))-1)
	assert.ErrorContains(t, err, "exceeds")

	// a layer whose uncompressed content is larger than its size is cut off, and what was read is returned
	limit := int64(4)
	got, err = readLayer(understatedLayer{Layer: static.NewLayer(data, types.MediaType("application/json"))}, limit)
	assert.ErrorContains(t, err, "uncompressed layer size exceeds")
	assert.Equal(t, int64(len(got)), limit+1)
}

func TestFetchBundlesFallbackLayerFetchError(t *testing.T) {
	t.Parallel()
	ref := setupFallbackRegistry(t, []testReferrer{{
		layerMediaType:  "application/json",
		layerData:       []byte(`{"hello":"world"}`),
		skipLayerUpload: true,
	}})
	_, _, err := fetchBundles(context.Background(), ref, attestationlimit, "", nil, nil)
	assert.ErrorContains(t, err, "failed to fetch referrer")
}

func TestFetchBundlesFallbackFetchError(t *testing.T) {
	t.Parallel()
	// the registry refuses to serve the referrer manifest, as when access is denied
	var blocked atomic.Pointer[string]
	handler := registry.New(registry.Logger(log.New(io.Discard, "", 0)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if digest := blocked.Load(); digest != nil && r.Method != http.MethodPut && strings.HasSuffix(r.URL.Path, "/manifests/"+*digest) {
			http.Error(w, "denied", http.StatusForbidden)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	repo, err := name.NewRepository(strings.TrimPrefix(server.URL, "http://") + "/test/app")
	assert.NilError(t, err)
	img, err := random.Image(64, 1)
	assert.NilError(t, err)
	ref := repo.Tag("latest")
	assert.NilError(t, remote.Write(ref, img))
	imgDigest, err := img.Digest()
	assert.NilError(t, err)

	desc := pushReferrer(t, repo, testReferrer{layerMediaType: "application/json", layerData: []byte(`{}`)})
	index, err := json.Marshal(v1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     types.OCIImageIndex,
		Manifests:     []v1.Descriptor{desc},
	})
	assert.NilError(t, err)
	fallbackTag := repo.Tag(fmt.Sprintf("%s-%s", imgDigest.Algorithm, imgDigest.Hex))
	assert.NilError(t, remote.Put(fallbackTag, rawManifest{body: index, mediaType: types.OCIImageIndex}))
	digest := desc.Digest.String()
	blocked.Store(&digest)

	_, _, err = fetchBundles(context.Background(), ref, attestationlimit, "", nil, nil)
	assert.ErrorContains(t, err, "failed to fetch referrer image")
}

// understatedLayer reports a zero size, like a referrer whose descriptor does not bound its content.
type understatedLayer struct {
	v1.Layer
}

func (understatedLayer) Size() (int64, error) { return 0, nil }
