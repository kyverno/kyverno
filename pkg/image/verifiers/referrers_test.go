package verifiers

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeIndex serves a fixed raw manifest, standing in for a referrers index the
// registry has already handed us: go-containerregistry buffers the body inside
// Referrers(), so the bound has to apply before IndexManifest unmarshals it.
type fakeIndex struct {
	raw []byte
}

func (f *fakeIndex) RawManifest() ([]byte, error) { return f.raw, nil }

func (f *fakeIndex) IndexManifest() (*v1.IndexManifest, error) {
	m := &v1.IndexManifest{}
	if err := json.Unmarshal(f.raw, m); err != nil {
		return nil, err
	}
	return m, nil
}

// indexWithPadding builds a syntactically valid index whose body is at least n bytes.
func indexWithPadding(t *testing.T, n int) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     "application/vnd.oci.image.index.v1+json",
		"manifests":     []any{},
		"annotations":   map[string]string{"pad": strings.Repeat("A", n)},
	})
	require.NoError(t, err)
	return raw
}

func TestReferrersIndexManifest_WithinLimit(t *testing.T) {
	m, err := ReferrersIndexManifest(&fakeIndex{raw: indexWithPadding(t, 1024)})
	require.NoError(t, err)
	assert.NotNil(t, m)
}

// An oversized index must be refused before it is unmarshalled: the unmarshal is
// where the cost lands, roughly 10x the body in Go structs.
func TestReferrersIndexManifest_OversizedIsRefused(t *testing.T) {
	raw := indexWithPadding(t, int(maxReferrersIndexSize)+1)
	_, err := ReferrersIndexManifest(&fakeIndex{raw: raw})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "referrers index size")
	assert.Contains(t, err.Error(), "exceeds")
}

// A legitimate index of maxReferrersCount real descriptors must fit with headroom.
func TestReferrersIndexManifest_RealisticIndexFits(t *testing.T) {
	manifests := make([]v1.Descriptor, 0, 50)
	for i := 0; i < 50; i++ {
		img, err := random.Image(64, 1)
		require.NoError(t, err)
		d, err := img.Digest()
		require.NoError(t, err)
		manifests = append(manifests, v1.Descriptor{
			MediaType:    "application/vnd.oci.image.manifest.v1+json",
			Digest:       d,
			Size:         1024,
			ArtifactType: "application/vnd.cncf.notary.signature",
			Annotations:  map[string]string{"org.opencontainers.image.created": fmt.Sprintf("2026-10-%02dT00:00:00Z", i%28+1)},
		})
	}
	raw, err := json.Marshal(v1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.index.v1+json",
		Manifests:     manifests,
	})
	require.NoError(t, err)
	require.Less(t, int64(len(raw)), maxReferrersIndexSize, "a 50-entry index must sit well under the bound")

	m, err := ReferrersIndexManifest(&fakeIndex{raw: raw})
	require.NoError(t, err)
	assert.Len(t, m.Manifests, 50)
}
