package bundle

import (
	"bytes"
	"encoding/json"
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
)

// image is a minimal v1.Image implementation for a format 1.0 bundle. go-containerregistry's
// mutate package (mutate.Config, mutate.ConfigFile, mutate.ConfigMediaType) has no way to set a
// config blob's raw bytes to anything other than a marshaled v1.ConfigFile — it recomputes the
// manifest's config descriptor only for a recognized Docker/OCI image config media type, and
// otherwise leaves the base image's config descriptor untouched. Since the bundle's config isn't
// that shape, image builds the manifest itself instead of going through mutate.
type image struct {
	layer       v1.Layer
	configBytes []byte
	manifest    *v1.Manifest
	rawManifest []byte
}

var _ v1.Image = (*image)(nil)

// newImage builds the v1.Image for a manifest with exactly one layer (bundle-spec.md section 4:
// an OCI 1.0 image manifest, no artifactType, no subject) and the given config bytes.
func newImage(layer v1.Layer, configBytes []byte, annotations map[string]string) (v1.Image, error) {
	layerDigest, err := layer.Digest()
	if err != nil {
		return nil, fmt.Errorf("digesting content layer: %w", err)
	}
	layerSize, err := layer.Size()
	if err != nil {
		return nil, fmt.Errorf("sizing content layer: %w", err)
	}
	layerMediaType, err := layer.MediaType()
	if err != nil {
		return nil, fmt.Errorf("getting content layer media type: %w", err)
	}

	configDigest, configSize, err := v1.SHA256(bytes.NewReader(configBytes))
	if err != nil {
		return nil, fmt.Errorf("digesting config: %w", err)
	}

	manifest := &v1.Manifest{
		SchemaVersion: 2,
		MediaType:     types.OCIManifestSchema1,
		Config: v1.Descriptor{
			MediaType: types.MediaType(internal.ConfigMediaType),
			Digest:    configDigest,
			Size:      configSize,
		},
		Layers: []v1.Descriptor{{
			MediaType: layerMediaType,
			Digest:    layerDigest,
			Size:      layerSize,
		}},
		Annotations: annotations,
	}

	rawManifest, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("marshaling manifest: %w", err)
	}

	return &image{
		layer:       layer,
		configBytes: configBytes,
		manifest:    manifest,
		rawManifest: rawManifest,
	}, nil
}

func (i *image) Layers() ([]v1.Layer, error) { return []v1.Layer{i.layer}, nil }

func (i *image) MediaType() (types.MediaType, error) { return i.manifest.MediaType, nil }

func (i *image) Size() (int64, error) { return int64(len(i.rawManifest)), nil }

func (i *image) ConfigName() (v1.Hash, error) { return i.manifest.Config.Digest, nil }

// ConfigFile parses the config bytes as a v1.ConfigFile for interface compliance. The bundle's
// config isn't a Docker/OCI image config, so this is best-effort and unused by the format;
// RawConfigFile is the config a reader actually wants.
func (i *image) ConfigFile() (*v1.ConfigFile, error) {
	return partial.ConfigFile(i)
}

func (i *image) RawConfigFile() ([]byte, error) { return i.configBytes, nil }

func (i *image) Digest() (v1.Hash, error) {
	h, _, err := v1.SHA256(bytes.NewReader(i.rawManifest))
	return h, err
}

func (i *image) Manifest() (*v1.Manifest, error) { return i.manifest.DeepCopy(), nil }

func (i *image) RawManifest() ([]byte, error) { return i.rawManifest, nil }

func (i *image) LayerByDigest(h v1.Hash) (v1.Layer, error) {
	d, err := i.layer.Digest()
	if err != nil {
		return nil, err
	}
	if d == h {
		return i.layer, nil
	}
	return nil, fmt.Errorf("layer not found: %s", h)
}

func (i *image) LayerByDiffID(h v1.Hash) (v1.Layer, error) {
	d, err := i.layer.DiffID()
	if err != nil {
		return nil, err
	}
	if d == h {
		return i.layer, nil
	}
	return nil, fmt.Errorf("layer not found: %s", h)
}
