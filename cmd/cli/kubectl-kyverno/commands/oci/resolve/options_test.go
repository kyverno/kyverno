package resolve

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	gcrname "github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
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

func pushBundle(t *testing.T, host, ref string) (v1.Image, string) {
	t.Helper()
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := bundle.Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, bundle.Validate(b))
	img, err := bundle.Write(b, nil)
	require.NoError(t, err)

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
