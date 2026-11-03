package bundle

import (
	"encoding/json"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
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

// legacyConfigImage wraps *image to override Manifest() with a hand-mutated copy, since *image's
// own fields are private to this package and newImage always builds a manifest consistent with
// its inputs; the malformed-shape tests above need a manifest that disagrees with what newImage
// would otherwise produce.
type legacyConfigImage struct {
	*image
	manifest *v1.Manifest
}

func (i *legacyConfigImage) Manifest() (*v1.Manifest, error) { return i.manifest.DeepCopy(), nil }
