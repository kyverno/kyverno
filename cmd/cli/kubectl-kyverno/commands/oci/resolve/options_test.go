package resolve

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	gcrname "github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal/bundle"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal/ocitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o750))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o600))
	}
	return dir
}

const validVP = `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: require-labels
spec:
  validations:
  - expression: "true"
`

// buildBundleImage assembles a minimal valid bundle without pushing it, so tests that need to
// patch its config or layers before pushing can start from a known-good image.
func buildBundleImage(t *testing.T) v1.Image {
	t.Helper()
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := bundle.Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, bundle.Validate(b))
	img, err := bundle.Write(b, nil)
	require.NoError(t, err)
	return img
}

func pushBundle(t *testing.T, host, ref string) (v1.Image, string) {
	t.Helper()
	img := buildBundleImage(t)

	full := host + "/" + ref
	name, err := gcrname.ParseReference(full)
	require.NoError(t, err)
	require.NoError(t, remote.Write(name, img))
	return img, full
}

// TestExecuteResolvesTagToPinnedReference pins D1's default output: the fully qualified
// digest-pinned reference, matching the image's own digest.
func TestExecuteResolvesTagToPinnedReference(t *testing.T) {
	host := ocitest.NewRegistry(t)
	img, ref := pushBundle(t, host, "bundles/baseline:v1")
	wantDigest, err := img.Digest()
	require.NoError(t, err)

	var out bytes.Buffer
	o := options{imageRef: ref}
	require.NoError(t, o.execute(context.Background(), &out, nil))
	assert.Equal(t, host+"/bundles/baseline@"+wantDigest.String()+"\n", out.String())
}

// TestExecuteDigestOnlyPrintsBareDigest pins --digest-only's output.
func TestExecuteDigestOnlyPrintsBareDigest(t *testing.T) {
	host := ocitest.NewRegistry(t)
	img, ref := pushBundle(t, host, "bundles/baseline:v1")
	wantDigest, err := img.Digest()
	require.NoError(t, err)

	var out bytes.Buffer
	o := options{imageRef: ref, digestOnly: true}
	require.NoError(t, o.execute(context.Background(), &out, nil))
	assert.Equal(t, wantDigest.String()+"\n", out.String())
}

// TestExecuteDigestInputResolvesToItself pins D1: a digest reference as input resolves to
// itself, after the existence and shape check.
func TestExecuteDigestInputResolvesToItself(t *testing.T) {
	host := ocitest.NewRegistry(t)
	img, _ := pushBundle(t, host, "bundles/baseline:v1")
	wantDigest, err := img.Digest()
	require.NoError(t, err)

	digestRef := host + "/bundles/baseline@" + wantDigest.String()
	var out bytes.Buffer
	o := options{imageRef: digestRef}
	require.NoError(t, o.execute(context.Background(), &out, nil))
	assert.Equal(t, digestRef+"\n", out.String())
}

// TestExecuteRejectsNonBundleImage pins the design-critic's amendment: resolve's non-bundle
// error names the "resolve only pins Kyverno bundles" wrapper and points at "crane digest",
// without printing the digest.
func TestExecuteRejectsNonBundleImage(t *testing.T) {
	host := ocitest.NewRegistry(t)
	// mutate.MediaType forces the OCI 1.0 image manifest media type FetchDescriptor's guard
	// requires, so this exercises resolve's own CheckManifestShape rejection (wrong config/layer
	// media type), not the shared guard's "got a Docker schema-2 manifest" case.
	base, err := random.Image(256, 1)
	require.NoError(t, err)
	img := mutate.MediaType(base, types.OCIManifestSchema1)
	ref := host + "/plain/image:v1"
	name, err := gcrname.ParseReference(ref)
	require.NoError(t, err)
	require.NoError(t, remote.Write(name, img))

	var out bytes.Buffer
	o := options{imageRef: ref}
	err = o.execute(context.Background(), &out, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve only pins Kyverno bundles:")
	assert.Contains(t, err.Error(), "crane digest")
	assert.Empty(t, out.String())
}

// TestExecuteRejectsLegacyMediaType pins D1's shape check wrapping a pre-1.20 image's "legacy
// image format" error inside "resolve only pins Kyverno bundles:".
func TestExecuteRejectsLegacyMediaType(t *testing.T) {
	host := ocitest.NewRegistry(t)
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := bundle.Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, bundle.Validate(b))
	img, err := bundle.Write(b, nil)
	require.NoError(t, err)
	legacyImg := mutate.ConfigMediaType(img, types.MediaType(internal.LegacyConfigMediaType))

	ref := host + "/bundles/legacy:v1"
	name, err := gcrname.ParseReference(ref)
	require.NoError(t, err)
	require.NoError(t, remote.Write(name, legacyImg))

	var out bytes.Buffer
	o := options{imageRef: ref}
	err = o.execute(context.Background(), &out, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve only pins Kyverno bundles:")
	assert.Contains(t, err.Error(), "legacy image format")
	assert.Empty(t, out.String())
}

// pushWithConfigBytes pushes base's single content layer unchanged but replaces its config blob's
// bytes with configBytes verbatim, updating the manifest's config digest/size to match. This lets
// a test push a config the typed bundle.Config/mutate package can't produce, e.g. non-JSON bytes
// or a format version this reader rejects.
func pushWithConfigBytes(t *testing.T, host, ref string, base v1.Image, configBytes []byte) {
	t.Helper()
	baseManifest, err := base.Manifest()
	require.NoError(t, err)
	layers, err := base.Layers()
	require.NoError(t, err)
	require.Len(t, layers, 1)

	configLayer := static.NewLayer(configBytes, types.MediaType(internal.ConfigMediaType))
	configDigest, err := configLayer.Digest()
	require.NoError(t, err)
	configSize, err := configLayer.Size()
	require.NoError(t, err)

	manifest := baseManifest.DeepCopy()
	manifest.Config.Digest = configDigest
	manifest.Config.Size = configSize
	manifestBytes, err := json.Marshal(manifest)
	require.NoError(t, err)

	full := host + "/" + ref
	refName, err := gcrname.ParseReference(full)
	require.NoError(t, err)
	repo := refName.Context()

	require.NoError(t, remote.WriteLayer(repo, layers[0]))
	require.NoError(t, remote.WriteLayer(repo, configLayer))
	require.NoError(t, remote.Put(refName, manifestTaggable{raw: manifestBytes, mt: manifest.MediaType}))
}

// manifestTaggable is the minimal remote.Taggable so a hand-built manifest can be pushed with
// remote.Put without constructing a full v1.Image.
type manifestTaggable struct {
	raw []byte
	mt  types.MediaType
}

func (m manifestTaggable) RawManifest() ([]byte, error)        { return m.raw, nil }
func (m manifestTaggable) MediaType() (types.MediaType, error) { return m.mt, nil }

// TestExecuteRejectsNonJSONConfig pins the Copilot-reported gap: a manifest with bundle media
// types whose config blob isn't even valid JSON must still be refused, which requires resolve to
// read the config (bundle.ReadMetadata), not merely check the manifest's shape.
func TestExecuteRejectsNonJSONConfig(t *testing.T) {
	host := ocitest.NewRegistry(t)
	base := buildBundleImage(t)
	ref := "bundles/bad-config:v1"
	pushWithConfigBytes(t, host, ref, base, []byte("not json at all"))

	var out bytes.Buffer
	o := options{imageRef: host + "/" + ref}
	err := o.execute(context.Background(), &out, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve only pins Kyverno bundles:")
	assert.Empty(t, out.String())
}

// TestExecuteRejectsUnsupportedFormatMajorVersion pins the same gap for a config whose
// formatVersion disagrees with what this reader supports, even though the manifest's media types
// are otherwise well-formed format-1 bundle media types.
func TestExecuteRejectsUnsupportedFormatMajorVersion(t *testing.T) {
	host := ocitest.NewRegistry(t)
	base := buildBundleImage(t)
	raw, err := base.RawConfigFile()
	require.NoError(t, err)
	var cfg map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &cfg))
	cfg["formatVersion"] = "2.0.0"
	patched, err := json.Marshal(cfg)
	require.NoError(t, err)

	ref := "bundles/future-major:v1"
	pushWithConfigBytes(t, host, ref, base, patched)

	var out bytes.Buffer
	o := options{imageRef: host + "/" + ref}
	err = o.execute(context.Background(), &out, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve only pins Kyverno bundles:")
	assert.Contains(t, err.Error(), "unsupported bundle format major version")
	assert.Empty(t, out.String())
}

// TestExecuteRejectsIndexReference pins the shared FetchDescriptor guard (D1, decision-log
// "index guard"): a reference whose manifest is an image index fails with the shared
// "expected an OCI 1.0 image manifest" error.
func TestExecuteRejectsIndexReference(t *testing.T) {
	host := ocitest.NewRegistry(t)
	img, err := random.Image(256, 1)
	require.NoError(t, err)
	idx := mutate.AppendManifests(empty.Index, mutate.IndexAddendum{
		Add: img,
		Descriptor: v1.Descriptor{
			Platform: &v1.Platform{OS: "linux", Architecture: "amd64"},
		},
	})
	ref := host + "/plain/indexed:v1"
	name, err := gcrname.ParseReference(ref)
	require.NoError(t, err)
	require.NoError(t, remote.WriteIndex(name, idx))

	var out bytes.Buffer
	o := options{imageRef: ref}
	err = o.execute(context.Background(), &out, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected an OCI 1.0 image manifest")
	assert.Empty(t, out.String())
}

// TestExecuteFailsOnUnknownTag pins that an unknown reference fails with no stdout output.
func TestExecuteFailsOnUnknownTag(t *testing.T) {
	host := ocitest.NewRegistry(t)
	var out bytes.Buffer
	o := options{imageRef: host + "/does-not-exist/repo:v1"}
	err := o.execute(context.Background(), &out, nil)
	assert.Error(t, err)
	assert.Empty(t, out.String())
}

func TestValidateRequiresImage(t *testing.T) {
	o := options{}
	err := o.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image is required")
}
