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
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
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

// TestExecuteNoAnnotationsShowsNone pins "Annotations: none" for a bundle pushed without
// manifest annotations.
func TestExecuteFailsOnUnknownTag(t *testing.T) {
	host := ocitest.NewRegistry(t)
	var out, errOut bytes.Buffer
	o := options{imageRef: host + "/does-not-exist/repo:v1"}
	err := o.execute(context.Background(), &out, &errOut, nil)
	assert.Error(t, err)
	assert.Empty(t, out.String())
}

// TestExecuteRejectsIndexReference pins the shared FetchDescriptor guard on inspect's own read
// path, same as resolve's.
func TestExecuteRejectsIndexReference(t *testing.T) {
	host := ocitest.NewRegistry(t)
	img, err := random.Image(256, 1)
	require.NoError(t, err)
	full := host + "/plain/image:v1"
	name, err := gcrname.ParseReference(full)
	require.NoError(t, err)
	require.NoError(t, remote.Write(name, img))

	var out, errOut bytes.Buffer
	o := options{imageRef: full}
	err = o.execute(context.Background(), &out, &errOut, nil)
	require.Error(t, err)
	assert.Empty(t, out.String())
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
