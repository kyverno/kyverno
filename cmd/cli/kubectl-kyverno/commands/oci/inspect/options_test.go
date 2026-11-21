package inspect

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

// mutateAnnotation overrides img's format-version annotation to a value that disagrees with the
// config blob, producing exactly one annotationWarnings mismatch.
func mutateAnnotation(t *testing.T, img v1.Image) v1.Image {
	t.Helper()
	mutated, ok := mutate.Annotations(img, map[string]string{internal.AnnotationFormatVersion: "9.9.9"}).(v1.Image)
	require.True(t, ok)
	return mutated
}

// mustExtractConfig pulls the raw "config" field out of a marshaled Report.
func mustExtractConfig(t *testing.T, data []byte) json.RawMessage {
	t.Helper()
	var r Report
	require.NoError(t, json.Unmarshal(data, &r))
	return r.Config
}

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

const namespacedException = `apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: check-pod
  namespace: team-a
spec:
  policyRefs:
  - name: require-labels
    kind: ValidatingPolicy
`

const multiDocumentFirst = `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: multi-first
spec:
  validations:
  - expression: "true"
`

const multiDocument = multiDocumentFirst + "---\n" + `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: multi-second
spec:
  validations:
  - expression: "true"
`

// pushBundle assembles and writes a bundle tree, returning the fetched config and the fully
// qualified reference it was pushed to.
func pushBundle(t *testing.T, host, ref string, files map[string]string) bundle.Config {
	t.Helper()
	dir := writeTree(t, files)
	b, err := bundle.Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, bundle.Validate(b))
	img, err := bundle.Write(b, nil)
	require.NoError(t, err)

	full := host + "/" + ref
	name, err := gcrname.ParseReference(full)
	require.NoError(t, err)
	require.NoError(t, remote.Write(name, img))

	md, err := bundle.ReadMetadata(img)
	require.NoError(t, err)
	return md.Config
}

// TestExecuteTextModeRendersLayout pins D2's text layout end to end: sets, a namespaced
// exception's NAMESPACE column, and a multi-document file's "(document 1)" suffix.
func TestExecuteTextModeRendersLayout(t *testing.T) {
	host := ocitest.NewRegistry(t)
	pushBundle(t, host, "bundles/baseline:v1", map[string]string{
		"policies/require-labels.yaml":      validVP,
		"exceptions/team-a-exceptions.yaml": namespacedException,
		"multi.yaml":                        multiDocument,
	})

	var out, errOut bytes.Buffer
	o := options{imageRef: host + "/bundles/baseline:v1", kyvernoTarget: "1.20.0"}
	require.NoError(t, o.execute(context.Background(), &out, &errOut, nil))

	text := out.String()
	assert.Contains(t, text, "Reference:")
	assert.Contains(t, text, "Resources (4):")
	assert.Contains(t, text, "NAMESPACE")
	assert.Contains(t, text, "team-a")
	assert.Contains(t, text, "multi.yaml (document 1)")
	assert.Contains(t, text, "Sets:")
	assert.Empty(t, errOut.String())
}

// TestExecuteJSONRoundTripsConfig pins -o json's config bytes equal img.RawConfigFile().
func TestExecuteJSONRoundTripsConfig(t *testing.T) {
	host := ocitest.NewRegistry(t)
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := bundle.Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, bundle.Validate(b))
	img, err := bundle.Write(b, nil)
	require.NoError(t, err)
	raw, err := img.RawConfigFile()
	require.NoError(t, err)

	full := host + "/bundles/json:v1"
	name, err := gcrname.ParseReference(full)
	require.NoError(t, err)
	require.NoError(t, remote.Write(name, img))

	var out, errOut bytes.Buffer
	o := options{imageRef: full, output: "json", kyvernoTarget: "1.20.0"}
	require.NoError(t, o.execute(context.Background(), &out, &errOut, nil))
	assert.JSONEq(t, string(raw), string(mustExtractConfig(t, out.Bytes())))
	assert.Empty(t, errOut.String())
}

// TestExecuteYAMLModeParses pins -o yaml producing valid YAML.
func TestExecuteYAMLModeParses(t *testing.T) {
	host := ocitest.NewRegistry(t)
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := bundle.Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, bundle.Validate(b))
	img, err := bundle.Write(b, nil)
	require.NoError(t, err)

	full := host + "/bundles/yaml:v1"
	name, err := gcrname.ParseReference(full)
	require.NoError(t, err)
	require.NoError(t, remote.Write(name, img))

	var out, errOut bytes.Buffer
	o := options{imageRef: full, output: "yaml"}
	require.NoError(t, o.execute(context.Background(), &out, &errOut, nil))
	assert.Contains(t, out.String(), "digest:")
}

// TestExecuteCompatibilityStates pins satisfied/unsatisfied/unknown/undeclared via
// --kyverno-version against a config that declares kyvernoVersion.
func TestExecuteCompatibilityStates(t *testing.T) {
	host := ocitest.NewRegistry(t)
	dir := writeTree(t, map[string]string{
		"kyverno-bundle.yaml": `apiVersion: cli.kyverno.io/v1alpha1
kind: PolicyBundle
spec:
  kyvernoVersion: ">=1.20.0"
`,
		"policy.yaml": validVP,
	})
	b, err := bundle.Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, bundle.Validate(b))
	d, err := bundle.LoadDescriptor(dir, b.Documents)
	require.NoError(t, err)
	img, err := bundle.Write(b, d)
	require.NoError(t, err)
	full := host + "/bundles/compat:v1"
	name, err := gcrname.ParseReference(full)
	require.NoError(t, err)
	require.NoError(t, remote.Write(name, img))

	cases := []struct {
		target string
		want   string
	}{
		{"1.20.0", "satisfied"},
		{"1.0.0", "unsatisfied"},
		{"not-a-release", "unknown"},
	}
	for _, c := range cases {
		var out, errOut bytes.Buffer
		o := options{imageRef: full, output: "json", kyvernoTarget: c.target}
		require.NoError(t, o.execute(context.Background(), &out, &errOut, nil))
		assert.Contains(t, out.String(), `"result": "`+c.want+`"`)
	}
}

// TestExecuteUndeclaredCompatibility pins "undeclared" when the config has no kyvernoVersion.
func TestExecuteUndeclaredCompatibility(t *testing.T) {
	host := ocitest.NewRegistry(t)
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := bundle.Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, bundle.Validate(b))
	img, err := bundle.Write(b, nil)
	require.NoError(t, err)
	full := host + "/bundles/undeclared:v1"
	name, err := gcrname.ParseReference(full)
	require.NoError(t, err)
	require.NoError(t, remote.Write(name, img))

	var out, errOut bytes.Buffer
	o := options{imageRef: full, output: "json"}
	require.NoError(t, o.execute(context.Background(), &out, &errOut, nil))
	assert.Contains(t, out.String(), `"result": "undeclared"`)
}

// TestExecuteAnnotationMismatchWarnsOnStderrInTextMode and the JSON variant below pin D2's
// split: text mode prints "Warning:" lines to stderr with nothing on stdout changed; -o json
// carries the same string in warnings with an empty stderr, and both exit 0.
func TestExecuteAnnotationMismatchWarnsOnStderrInTextMode(t *testing.T) {
	host := ocitest.NewRegistry(t)
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := bundle.Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, bundle.Validate(b))
	img, err := bundle.Write(b, nil)
	require.NoError(t, err)
	full := host + "/bundles/mismatch:v1"
	name, err := gcrname.ParseReference(full)
	require.NoError(t, err)

	mutated := mutateAnnotation(t, img)
	require.NoError(t, remote.Write(name, mutated))

	var out, errOut bytes.Buffer
	o := options{imageRef: full}
	require.NoError(t, o.execute(context.Background(), &out, &errOut, nil))
	assert.Contains(t, errOut.String(), "Warning:")
	assert.Contains(t, errOut.String(), "is \"9.9.9\"")
}

func TestExecuteAnnotationMismatchWarnsInJSONField(t *testing.T) {
	host := ocitest.NewRegistry(t)
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := bundle.Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, bundle.Validate(b))
	img, err := bundle.Write(b, nil)
	require.NoError(t, err)
	full := host + "/bundles/mismatch-json:v1"
	name, err := gcrname.ParseReference(full)
	require.NoError(t, err)

	mutated := mutateAnnotation(t, img)
	require.NoError(t, remote.Write(name, mutated))

	var out, errOut bytes.Buffer
	o := options{imageRef: full, output: "json"}
	require.NoError(t, o.execute(context.Background(), &out, &errOut, nil))
	assert.Empty(t, errOut.String())
	assert.Contains(t, out.String(), `"warnings"`)
	assert.Contains(t, out.String(), "is \\\"9.9.9\\\"")
}

// TestExecuteFailsOnUnknownTag pins that resolving a tag that was never pushed is an error
// with no stdout output.
func TestExecuteFailsOnUnknownTag(t *testing.T) {
	host := ocitest.NewRegistry(t)
	var out, errOut bytes.Buffer
	o := options{imageRef: host + "/does-not-exist/repo:v1"}
	err := o.execute(context.Background(), &out, &errOut, nil)
	assert.Error(t, err)
	assert.Empty(t, out.String())
}

// TestExecuteRejectsIndexReference pins the shared FetchDescriptor guard on inspect's own read
// path, same as resolve's: a reference whose manifest is an image index fails with the shared
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
	full := host + "/plain/indexed:v1"
	name, err := gcrname.ParseReference(full)
	require.NoError(t, err)
	require.NoError(t, remote.WriteIndex(name, idx))

	var out, errOut bytes.Buffer
	o := options{imageRef: full}
	err = o.execute(context.Background(), &out, &errOut, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected an OCI 1.0 image manifest")
	assert.Empty(t, out.String())
}

func buildBundle(t *testing.T) v1.Image {
	t.Helper()
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := bundle.Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, bundle.Validate(b))
	img, err := bundle.Write(b, nil)
	require.NoError(t, err)
	return img
}

// pushWithPatchedConfig pushes base's single content layer unchanged, but replaces its config
// blob's bytes with configBytes (patch applied to the decoded JSON, re-marshaled), updating the
// manifest's config digest/size to match. Neither bundle.Config's typed struct nor
// go-containerregistry's mutate package can produce config bytes outside the shape they already
// know, so the config and content-layer blobs are pushed directly (remote.WriteLayer, remote.Put)
// instead of through remote.Write(ref, image).
func pushWithPatchedConfig(t *testing.T, host, ref string, base v1.Image, patch func(cfg map[string]interface{})) {
	t.Helper()
	baseManifest, err := base.Manifest()
	require.NoError(t, err)
	layers, err := base.Layers()
	require.NoError(t, err)
	require.Len(t, layers, 1)

	raw, err := base.RawConfigFile()
	require.NoError(t, err)
	var cfg map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &cfg))
	patch(cfg)
	patchedConfig, err := json.Marshal(cfg)
	require.NoError(t, err)

	configLayer := static.NewLayer(patchedConfig, types.MediaType(internal.ConfigMediaType))
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

// TestExecuteWrapsReaderErrors pins that execute surfaces bundle.ReadMetadata's own reader-MUST
// errors unwrapped, for every shape the reader rejects: a legacy pre-1.20 config media type, a
// manifest with more than one layer, and an unsupported format major version.
func TestExecuteWrapsReaderErrors(t *testing.T) {
	cases := []struct {
		name    string
		push    func(t *testing.T, host, ref string)
		wantErr string
	}{
		{
			name: "legacy media type",
			push: func(t *testing.T, host, ref string) {
				img := mutate.ConfigMediaType(buildBundle(t), types.MediaType(internal.LegacyConfigMediaType))
				full := host + "/" + ref
				name, err := gcrname.ParseReference(full)
				require.NoError(t, err)
				require.NoError(t, remote.Write(name, img))
			},
			wantErr: "legacy image format",
		},
		{
			name: "two layers",
			push: func(t *testing.T, host, ref string) {
				extra, err := random.Layer(16, types.OCILayer)
				require.NoError(t, err)
				img, err := mutate.AppendLayers(buildBundle(t), extra)
				require.NoError(t, err)
				full := host + "/" + ref
				name, err := gcrname.ParseReference(full)
				require.NoError(t, err)
				require.NoError(t, remote.Write(name, img))
			},
			wantErr: "malformed bundle",
		},
		{
			name: "wrong format major",
			push: func(t *testing.T, host, ref string) {
				pushWithPatchedConfig(t, host, ref, buildBundle(t), func(cfg map[string]interface{}) {
					cfg["formatVersion"] = "2.0.0"
				})
			},
			wantErr: "unsupported bundle format major version 2",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			host := ocitest.NewRegistry(t)
			ref := "bundles/bad:v1"
			c.push(t, host, ref)

			var out, errOut bytes.Buffer
			o := options{imageRef: host + "/" + ref}
			err := o.execute(context.Background(), &out, &errOut, nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantErr)
			assert.Empty(t, out.String())
		})
	}
}

// manifestTaggable is the minimal remote.Taggable so a hand-built manifest can be pushed with
// remote.Put without constructing a full v1.Image (which, per bundle/image.go, the mutate package
// can't do for a config blob that isn't a marshaled v1.ConfigFile).
type manifestTaggable struct {
	raw []byte
	mt  types.MediaType
}

func (m manifestTaggable) RawManifest() ([]byte, error)        { return m.raw, nil }
func (m manifestTaggable) MediaType() (types.MediaType, error) { return m.mt, nil }

// TestExecuteJSONPreservesUnknownConfigField pins D2's note that Report.Config is json.RawMessage:
// a config field a future minor version adds survives -o json untouched, even though this
// reader's typed bundle.Config doesn't know about it.
func TestExecuteJSONPreservesUnknownConfigField(t *testing.T) {
	host := ocitest.NewRegistry(t)
	ref := "bundles/future-field:v1"
	pushWithPatchedConfig(t, host, ref, buildBundle(t), func(cfg map[string]interface{}) {
		cfg["futureField"] = "from-a-later-minor"
	})

	var out, errOut bytes.Buffer
	o := options{imageRef: host + "/" + ref, output: "json"}
	require.NoError(t, o.execute(context.Background(), &out, &errOut, nil))
	assert.Empty(t, errOut.String())
	got := mustExtractConfig(t, out.Bytes())
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(got, &decoded))
	assert.Equal(t, "from-a-later-minor", decoded["futureField"])
}

func TestValidateRejectsNonReleaseKyvernoVersion(t *testing.T) {
	o := options{imageRef: "x", kyvernoTarget: "latest"}
	err := o.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a release version")
}

func TestValidateRejectsInvalidOutput(t *testing.T) {
	o := options{imageRef: "x", output: "xml"}
	err := o.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid output format")
}

func TestValidateRequiresImage(t *testing.T) {
	o := options{}
	err := o.validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image is required")
}
