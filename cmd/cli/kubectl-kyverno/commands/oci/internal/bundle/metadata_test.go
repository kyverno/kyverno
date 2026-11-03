package bundle

import (
	"encoding/json"
	"strings"
	"testing"

	gcrname "github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal/ocitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadMetadataReturnsConfigAndRawBytes pins D2's read path: ReadMetadata parses the config
// without extracting the content layer, and RawConfig is the blob's bytes verbatim.
func TestReadMetadataReturnsConfigAndRawBytes(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))
	img, err := Write(b, nil)
	require.NoError(t, err)

	md, err := ReadMetadata(img)
	require.NoError(t, err)
	assert.Equal(t, internal.FormatVersion, md.Config.FormatVersion)
	assert.Len(t, md.Config.Resources, 1)

	wantRaw, err := img.RawConfigFile()
	require.NoError(t, err)
	assert.Equal(t, wantRaw, md.RawConfig)

	assert.NotNil(t, md.Manifest)
	assert.Equal(t, types.OCIManifestSchema1, md.Manifest.MediaType)
}

// TestReadMetadataRejectsLegacyConfigMediaType pins reader MUST 1 over ReadMetadata's own call to
// CheckManifestShape, shared with Read.
func TestReadMetadataRejectsLegacyConfigMediaType(t *testing.T) {
	layer, err := buildContentLayer(map[string][]byte{"policy.yaml": []byte(validVP)})
	require.NoError(t, err)
	img, err := newImage(layer, []byte(`{}`), nil)
	require.NoError(t, err)

	manifest, err := img.Manifest()
	require.NoError(t, err)
	manifest.Config.MediaType = types.MediaType(internal.LegacyConfigMediaType)
	badImg := &legacyConfigImage{image: img.(*image), manifest: manifest}

	_, err = ReadMetadata(badImg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "legacy image format")
}

// TestReadMetadataRejectsTwoLayers pins reader MUST 1's layer-count check.
func TestReadMetadataRejectsTwoLayers(t *testing.T) {
	layer, err := buildContentLayer(map[string][]byte{"policy.yaml": []byte(validVP)})
	require.NoError(t, err)
	img, err := newImage(layer, []byte(`{"formatVersion":"1.0.0"}`), nil)
	require.NoError(t, err)

	manifest, err := img.Manifest()
	require.NoError(t, err)
	manifest.Layers = append(manifest.Layers, manifest.Layers[0])
	badImg := &legacyConfigImage{image: img.(*image), manifest: manifest}

	_, err = ReadMetadata(badImg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "malformed bundle")
	assert.Contains(t, err.Error(), "exactly one layer")
}

// TestReadMetadataRejectsUnsupportedFormatMajor pins reader MUST 6: a config declaring a format
// major version this reader doesn't support is rejected, naming both versions.
func TestReadMetadataRejectsUnsupportedFormatMajor(t *testing.T) {
	layer, err := buildContentLayer(map[string][]byte{"policy.yaml": []byte(validVP)})
	require.NoError(t, err)
	cfg := Config{FormatVersion: "2.0.0", Sets: []ConfigSet{}, Resources: []ResourceEntry{}}
	configBytes, err := json.Marshal(cfg)
	require.NoError(t, err)
	img, err := newImage(layer, configBytes, nil)
	require.NoError(t, err)

	_, err = ReadMetadata(img)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported bundle format major version 2")
}

// TestReadMetadataRejectsNonJSONConfig pins parseConfig's shared error wording for a config blob
// that isn't valid JSON.
func TestReadMetadataRejectsNonJSONConfig(t *testing.T) {
	layer, err := buildContentLayer(map[string][]byte{"policy.yaml": []byte(validVP)})
	require.NoError(t, err)
	img, err := newImage(layer, []byte("not json"), nil)
	require.NoError(t, err)

	_, err = ReadMetadata(img)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing config")
}

// TestReadMetadataDoesNotFetchContentLayer pins the reason ReadMetadata exists alongside Read: it
// reads the manifest and config only. A remote.Image is lazy, so this round-trips through a real
// in-process registry and asserts the content layer's blob was never requested; a ReadMetadata
// that called Read (or otherwise extracted the layer) would fail this test.
func TestReadMetadataDoesNotFetchContentLayer(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))
	img, err := Write(b, nil)
	require.NoError(t, err)

	manifest, err := img.Manifest()
	require.NoError(t, err)
	require.Len(t, manifest.Layers, 1)
	layerDigest := manifest.Layers[0].Digest.String()

	host, requests, reset := ocitest.NewRecordingRegistry(t)
	ref := host + "/bundles/metadata-only:v1"
	name, err := gcrname.ParseReference(ref)
	require.NoError(t, err)
	require.NoError(t, remote.Write(name, img))

	// Reset after the push: pushing its own blob-existence checks and uploads would otherwise
	// show up in the log we're about to inspect, which must only reflect the read below.
	reset()

	remoteImg, err := remote.Image(name)
	require.NoError(t, err)

	md, err := ReadMetadata(remoteImg)
	require.NoError(t, err)
	assert.Equal(t, internal.FormatVersion, md.Config.FormatVersion)

	seenRequests := requests()
	foundBlobRequest := false
	for _, path := range seenRequests {
		assert.NotContains(t, path, layerDigest, "ReadMetadata must not fetch the content layer blob, got request %q", path)
		if strings.Contains(path, "/blobs/") {
			foundBlobRequest = true
		}
	}
	// Sanity check the assertion isn't vacuous: ReadMetadata must fetch the config blob, so an
	// empty request log would mean the recorder itself is broken, not that nothing was fetched.
	assert.True(t, foundBlobRequest, "expected at least one blob request (the config fetch), recorder may be broken")
}

// legacyConfigImage wraps *image to override Manifest() with a hand-mutated copy, since *image's
// own fields are private to this package and newImage always builds a manifest consistent with
// its inputs; the malformed-shape tests above need a manifest that disagrees with what newImage
// would otherwise produce.
type legacyConfigImage struct {
	*image
	manifest *v1.Manifest
}

func (i *legacyConfigImage) Manifest() (*v1.Manifest, error) { return i.manifest.DeepCopy(), nil }
