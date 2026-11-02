package bundle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func descriptorYAML(body string) string {
	return "apiVersion: cli.kyverno.io/v1alpha1\nkind: PolicyBundle\n" + body
}

func set(name, typ string, paths ...string) v1alpha1.PolicyBundleSet {
	return v1alpha1.PolicyBundleSet{Name: name, Type: typ, Paths: paths}
}

func assignmentsByName(assignments []SetAssignment) map[string]string {
	m := map[string]string{}
	for _, a := range assignments {
		m[a.Name] = a.Set
	}
	return m
}

func TestLoadDescriptorReturnsNilWhenAbsent(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	d, err := LoadDescriptor(dir, nil)
	require.NoError(t, err)
	assert.Nil(t, d)
}

func TestLoadDescriptorReturnsNilForSingleFileInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	require.NoError(t, os.WriteFile(path, []byte(validVP), 0o600))

	d, err := LoadDescriptor(path, nil)
	require.NoError(t, err)
	assert.Nil(t, d)
}

func TestLoadDescriptorRejectsSymlinkedDescriptor(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"policy.yaml":      validVP,
		"real-bundle.yaml": descriptorYAML("metadata:\n  name: x\n"),
	})
	require.NoError(t, os.Symlink(filepath.Join(dir, "real-bundle.yaml"), filepath.Join(dir, internal.DescriptorFilename)))

	_, err := LoadDescriptor(dir, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a regular file")
}

func TestLoadDescriptorThroughSymlinkedRoot(t *testing.T) {
	realDir := writeTree(t, map[string]string{
		"policy.yaml":               validVP,
		internal.DescriptorFilename: descriptorYAML("metadata:\n  name: x\n"),
	})
	linkRoot := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(realDir, linkRoot))

	b, err := Assemble(linkRoot)
	require.NoError(t, err)
	d, err := LoadDescriptor(linkRoot, b.Documents)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, "x", d.Name)
}

func TestParseDescriptorRejectsWrongGVK(t *testing.T) {
	_, err := parseDescriptor([]byte("apiVersion: cli.kyverno.io/v1alpha1\nkind: Values\n"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected cli.kyverno.io/v1alpha1 PolicyBundle")
}

func TestParseDescriptorRejectsUnknownField(t *testing.T) {
	_, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\nspec:\n  unknownField: oops\n")))
	assert.Error(t, err)
}

func TestParseDescriptorRejectsMultipleDocuments(t *testing.T) {
	content := descriptorYAML("metadata:\n  name: x\n") + "\n---\n" + descriptorYAML("metadata:\n  name: y\n")
	_, err := parseDescriptor([]byte(content))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one document, found 2")
}

func TestParseDescriptorIgnoresMetadataNamespace(t *testing.T) {
	pb, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\n  namespace: ignored\n")))
	require.NoError(t, err)
	assert.Equal(t, "x", pb.Name)
}

func TestParseDescriptorRejectsUnquotedNumericKyvernoVersion(t *testing.T) {
	_, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\nspec:\n  kyvernoVersion: 1.20\n")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spec.kyvernoVersion")
	assert.Contains(t, err.Error(), "quote it")
}

func TestParseDescriptorRejectsUnquotedNumericVersion(t *testing.T) {
	_, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\nspec:\n  version: 1.10\n")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spec.version")
	assert.Contains(t, err.Error(), "quote it")
}

func TestParseDescriptorRejectsUnquotedNumericFormatVersion(t *testing.T) {
	_, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\nspec:\n  formatVersion: 1.0\n")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spec.formatVersion")
	assert.Contains(t, err.Error(), "quote it")
}

func TestParseDescriptorAcceptsQuotedNumericLookingVersions(t *testing.T) {
	pb, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\nspec:\n  version: \"1.10\"\n  kyvernoVersion: \"1.20\"\n")))
	require.NoError(t, err)
	assert.Equal(t, "1.10", pb.Spec.Version)
	assert.Equal(t, "1.20", pb.Spec.KyvernoVersion)
}

func TestParseDescriptorRejectsUnquotedNumericAnnotationValue(t *testing.T) {
	_, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\nspec:\n  annotations:\n    example.com/release: 1.20\n")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spec.annotations.example.com/release")
	assert.Contains(t, err.Error(), "quote it")
}

func TestParseDescriptorRejectsUnquotedNumericMetadataName(t *testing.T) {
	_, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: 1.20\n")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "metadata.name")
	assert.Contains(t, err.Error(), "quote it")
}

func TestParseDescriptorRejectsUnquotedNumericSetName(t *testing.T) {
	_, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\nspec:\n  sets:\n  - name: 1.0\n    type: policies\n    paths: [\"**\"]\n")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spec.sets[0].name")
	assert.Contains(t, err.Error(), "quote it")
}

func TestParseDescriptorRejectsOctalLookingSourceRevision(t *testing.T) {
	_, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\nspec:\n  source:\n    revision: 0123456\n")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spec.source.revision")
	assert.Contains(t, err.Error(), "quote it")
}

func TestParseDescriptorRejectsUnquotedNumericDescriptionAndSourceURL(t *testing.T) {
	_, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\nspec:\n  description: 1.20\n")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spec.description")

	_, err = parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\nspec:\n  source:\n    url: 1.20\n")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spec.source.url")
}

func TestParseDescriptorRejectsUnquotedNumericThroughAlias(t *testing.T) {
	// The anchor lives in metadata.labels, outside the checked scope on its own; only the alias
	// target, spec.version, is in scope. This fails if alias resolution were ever dropped, unlike
	// anchoring inside spec itself, which the gate would already reject regardless of aliasing.
	_, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\n  labels:\n    release: &v 1.20\nspec:\n  version: *v\n")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spec.version")
	assert.Contains(t, err.Error(), "quote it")
}

func TestParseDescriptorRejectsUnquotedNumericAnnotationKey(t *testing.T) {
	_, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\nspec:\n  annotations:\n    1.20: release\n")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spec.annotations")
	assert.Contains(t, err.Error(), "quote it")
}

func TestParseDescriptorRejectsBooleanLookingMetadataAnnotationKey(t *testing.T) {
	_, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\n  annotations:\n    yes: release\n")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "metadata.annotations")
	assert.Contains(t, err.Error(), "quote it")
}

func TestParseDescriptorErrorDoesNotSuggestTheCoercedValue(t *testing.T) {
	// For 1.20, suggesting the value back as "the quoted form" would be spec.version: "1.2" -- the
	// mangled reading, not what the author wrote. The message must name the field, not the value.
	_, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\nspec:\n  version: 1.20\n")))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "1.2\"")
}

func TestParseDescriptorRejectsUnquotedNumericThroughMergeKey(t *testing.T) {
	_, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\n  labels: &b\n    version: 1.20\nspec:\n  <<: *b\n")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spec.version")
	assert.Contains(t, err.Error(), "quote it")
}

func TestParseDescriptorRejectsUnquotedYAML11Boolean(t *testing.T) {
	_, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\nspec:\n  version: yes\n")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spec.version")
	assert.Contains(t, err.Error(), "quote it")
}

func TestParseDescriptorAcceptsUnquotedTimestampLookingKyvernoVersion(t *testing.T) {
	// sigs.k8s.io/yaml (via go.yaml.in/yaml/v2 into interface{}) keeps an unquoted date a plain
	// string, unlike go.yaml.in/yaml/v3's node API, which would tag it !!timestamp; the gate must
	// agree with the decoder actually used, not over-reject a value that was never corrupted.
	pb, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\nspec:\n  kyvernoVersion: 2026-01-01\n")))
	require.NoError(t, err)
	assert.Equal(t, "2026-01-01", pb.Spec.KyvernoVersion)
}

func TestLoadDescriptorTreatsDirectoryNamedDescriptorAsAbsent(t *testing.T) {
	dir := writeTree(t, map[string]string{
		filepath.Join(internal.DescriptorFilename, "policy.yaml"): validVP,
	})
	b, err := Assemble(dir)
	require.NoError(t, err)
	d, err := LoadDescriptor(dir, b.Documents)
	require.NoError(t, err)
	assert.Nil(t, d)
}

func TestResolveDescriptorCopiesMetadataFields(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)

	pb, err := parseDescriptor([]byte(descriptorYAML(`metadata:
  name: baseline
spec:
  version: "1.2.3"
  description: a test bundle
  kyvernoVersion: ">=1.19.0"
  source:
    url: https://example.com/repo
    revision: abc123
  annotations:
    example.com/team: team-a
`)))
	require.NoError(t, err)

	d, err := resolveDescriptor(pb, b.Documents)
	require.NoError(t, err)
	assert.Equal(t, "baseline", d.Name)
	assert.Equal(t, "1.2.3", d.Version)
	assert.Equal(t, "a test bundle", d.Description)
	assert.Equal(t, ">=1.19.0", d.KyvernoVersion)
	require.NotNil(t, d.Source)
	assert.Equal(t, "https://example.com/repo", d.Source.URL)
	assert.Equal(t, "abc123", d.Source.Revision)
	assert.Equal(t, "team-a", d.Annotations["example.com/team"])
}

func TestResolveDescriptorAnnotationsReachWriteAndReservedKeysAreStripped(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"policy.yaml": validVP,
		internal.DescriptorFilename: descriptorYAML(`metadata:
  name: baseline
spec:
  annotations:
    example.com/team: team-a
    io.kyverno.bundle.version: "9.9.9"
`),
	})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	d, err := LoadDescriptor(dir, b.Documents)
	require.NoError(t, err)
	require.NotNil(t, d)

	img, err := Write(b, d)
	require.NoError(t, err)
	manifest, err := img.Manifest()
	require.NoError(t, err)
	assert.Equal(t, "team-a", manifest.Annotations["example.com/team"])
	assert.NotEqual(t, "9.9.9", manifest.Annotations[internal.AnnotationVersion])
}

func TestResolveDescriptorRejectsFormatVersionPinMismatch(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)

	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{name: "exact match accepted", version: "1.0.0", wantErr: false},
		{name: "two-component rejected", version: "1.0", wantErr: true},
		{name: "mismatched major rejected", version: "2.0.0", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pb, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\nspec:\n  formatVersion: \"" + tc.version + "\"\n")))
			require.NoError(t, err)
			_, err = resolveDescriptor(pb, b.Documents)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "pins formatVersion")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResolveDescriptorRejectsNonRFC3339Created(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)

	pb, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\nspec:\n  created: \"2026-01-01\"\n")))
	require.NoError(t, err)
	_, err = resolveDescriptor(pb, b.Documents)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not an RFC 3339 timestamp")
}

func TestResolveDescriptorPassesCreatedThroughVerbatim(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)

	const ts = "2026-01-01T00:00:00Z"
	pb, err := parseDescriptor([]byte(descriptorYAML("metadata:\n  name: x\nspec:\n  created: \"" + ts + "\"\n")))
	require.NoError(t, err)
	d, err := resolveDescriptor(pb, b.Documents)
	require.NoError(t, err)
	require.NotNil(t, d.Created)
	assert.Equal(t, ts, *d.Created)
}

func TestDeriveSetsAssignsByGlob(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"policies/require-labels.yaml": validVP,
		"exceptions/exempt.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: exempt
spec:
  policyRefs:
  - name: require-labels
    kind: ValidatingPolicy
`,
	})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	assignments, err := deriveSets([]v1alpha1.PolicyBundleSet{
		set("policies", "policies", "policies/**"),
		set("exceptions", "exceptions", "exceptions/**"),
	}, b.Documents)
	require.NoError(t, err)
	byName := assignmentsByName(assignments)
	assert.Equal(t, "policies", byName["require-labels"])
	assert.Equal(t, "exceptions", byName["exempt"])
}

func TestDeriveSetsDoubleStarCrossesSeparatorAndSingleStarDoesNot(t *testing.T) {
	dir := writeTree(t, map[string]string{"a/b/policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	// "*" at the top level does not cross "/", so it can't claim a/b/policy.yaml; the set
	// declares a path that matches nothing, which is the no-claim error.
	_, err = deriveSets([]v1alpha1.PolicyBundleSet{set("team-a", "policies", "a/*")}, b.Documents)
	assert.Error(t, err, "'*' must not cross a path separator")
	assert.Contains(t, err.Error(), "claims no indexed document")

	// "**" does cross it.
	assignments, err := deriveSets([]v1alpha1.PolicyBundleSet{set("team-a", "policies", "a/**")}, b.Documents)
	require.NoError(t, err)
	assert.Len(t, assignments, 1)
}

func TestDeriveSetsDoubleStarSlashDoesNotMatchRootFile(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	_, err = deriveSets([]v1alpha1.PolicyBundleSet{set("team-a", "policies", "**/policy.yaml")}, b.Documents)
	assert.Error(t, err, "**/x.yaml is a documented edge: it never matches a root-level x.yaml")
	assert.Contains(t, err.Error(), "claims no indexed document")

	assignments, err := deriveSets([]v1alpha1.PolicyBundleSet{set("team-a", "policies", "policy.yaml", "**/policy.yaml")}, b.Documents)
	require.NoError(t, err)
	assert.Len(t, assignments, 1, "the recommended {x.yaml,**/x.yaml} workaround must match")
}

// TestDeriveSetsBraceAlternationWorkaroundMatchesRootAndNested pins the design's recommended
// single-pattern form "{x.yaml,**/x.yaml}" as a literal brace-alternation glob, distinct from the
// two-separate-paths form exercised by TestDeriveSetsDoubleStarSlashDoesNotMatchRootFile.
func TestDeriveSetsBraceAlternationWorkaroundMatchesRootAndNested(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"policy.yaml": validVP,
		"a/policy2.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: nested-policy
spec:
  validations:
  - expression: "true"
`,
	})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	assignments, err := deriveSets([]v1alpha1.PolicyBundleSet{set("team-a", "policies", "{policy.yaml,**/policy2.yaml}")}, b.Documents)
	require.NoError(t, err)
	byName := assignmentsByName(assignments)
	assert.Equal(t, "team-a", byName["require-labels"], "root-level policy.yaml must match the brace alternation's first arm")
	assert.Equal(t, "team-a", byName["nested-policy"], "nested a/policy2.yaml must match the brace alternation's **-prefixed arm")
}

func TestDeriveSetsBackslashIsEscapeNotSeparator(t *testing.T) {
	dir := writeTree(t, map[string]string{`weird\file.yaml`: validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	assignments, err := deriveSets([]v1alpha1.PolicyBundleSet{set("team-a", "policies", `weird\\file.yaml`)}, b.Documents)
	require.NoError(t, err)
	assert.Len(t, assignments, 1)
}

func TestDeriveSetsTypeFiltersMixedDocumentFile(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"policies/bundle.yaml": validVP + "\n---\n" + `apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: exempt
spec:
  policyRefs:
  - name: require-labels
    kind: ValidatingPolicy
`,
	})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	assignments, err := deriveSets([]v1alpha1.PolicyBundleSet{set("policies", "policies", "policies/**")}, b.Documents)
	require.NoError(t, err)
	byName := assignmentsByName(assignments)
	assert.Equal(t, "policies", byName["require-labels"])
	_, exceptionAssigned := byName["exempt"]
	assert.False(t, exceptionAssigned, "the exception falls through to the by-kind default, not the policies set")
}

func TestDeriveSetsTwoTypedSetsOverSameGlobPartitionTree(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"bundle.yaml": validVP + "\n---\n" + `apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: exempt
spec:
  policyRefs:
  - name: require-labels
    kind: ValidatingPolicy
`,
	})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	assignments, err := deriveSets([]v1alpha1.PolicyBundleSet{
		set("policies", "policies", "**"),
		set("exceptions", "exceptions", "**"),
	}, b.Documents)
	require.NoError(t, err)
	byName := assignmentsByName(assignments)
	assert.Equal(t, "policies", byName["require-labels"])
	assert.Equal(t, "exceptions", byName["exempt"])
}

func TestDeriveSetsUnmatchedResourceFallsBackToDefault(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"team-a/policy.yaml": validVP,
		"other/policy2.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: other-policy
spec:
  validations:
  - expression: "true"
`,
	})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	d := &Descriptor{}
	assignments, err := deriveSets([]v1alpha1.PolicyBundleSet{set("team-a", "policies", "team-a/**")}, b.Documents)
	require.NoError(t, err)
	d.Sets = assignments

	img, err := Write(b, d)
	require.NoError(t, err)
	configBytes, err := img.RawConfigFile()
	require.NoError(t, err)
	var cfg Config
	require.NoError(t, json.Unmarshal(configBytes, &cfg))

	byName := map[string]string{}
	for _, r := range cfg.Resources {
		byName[r.Name] = r.Set
	}
	assert.Equal(t, "team-a", byName["require-labels"])
	assert.Equal(t, "policies", byName["other-policy"], "unmatched resource falls back to the by-kind default")
}

func TestDeriveSetsRejectsResourceClaimedByTwoSetsOfSameType(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	_, err = deriveSets([]v1alpha1.PolicyBundleSet{
		set("team-a", "policies", "**"),
		set("team-b", "policies", "**"),
	}, b.Documents)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "claimed by more than one set")
}

func TestDeriveSetsAllowsResourceMatchingTwoPathsOfSameSet(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	assignments, err := deriveSets([]v1alpha1.PolicyBundleSet{set("policies", "policies", "policy.yaml", "**")}, b.Documents)
	require.NoError(t, err)
	assert.Len(t, assignments, 1)
}

func TestDeriveSetsRejectsReservedNameWithWrongType(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	_, err = deriveSets([]v1alpha1.PolicyBundleSet{set("policies", "exceptions", "**")}, b.Documents)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reserved default set name")
}

func TestDeriveSetsRejectsDuplicateSetName(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	_, err = deriveSets([]v1alpha1.PolicyBundleSet{
		set("team-a", "policies", "**"),
		set("team-a", "policies", "**"),
	}, b.Documents)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate set name")
}

func TestDeriveSetsRejectsUnknownType(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	_, err = deriveSets([]v1alpha1.PolicyBundleSet{set("team-a", "widgets", "**")}, b.Documents)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `must be "policies" or "exceptions"`)
}

func TestDeriveSetsRejectsCapitalizedType(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	_, err = deriveSets([]v1alpha1.PolicyBundleSet{set("team-a", "Policies", "**")}, b.Documents)
	assert.Error(t, err)
}

func TestDeriveSetsRejectsEmptyPaths(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	_, err = deriveSets([]v1alpha1.PolicyBundleSet{set("team-a", "policies")}, b.Documents)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "declares no paths")
}

func TestDeriveSetsRejectsAbsolutePath(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	_, err = deriveSets([]v1alpha1.PolicyBundleSet{set("team-a", "policies", "/abs/**")}, b.Documents)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be relative to the bundle root")
}

func TestDeriveSetsRejectsDotSlashPath(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	_, err = deriveSets([]v1alpha1.PolicyBundleSet{set("team-a", "policies", "./x/**")}, b.Documents)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `must not start with "./"`)
}

func TestDeriveSetsRejectsTrailingSlashPath(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	_, err = deriveSets([]v1alpha1.PolicyBundleSet{set("team-a", "policies", "x/")}, b.Documents)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `must not end with "/"`)
}

func TestDeriveSetsRejectsInvalidGlob(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	_, err = deriveSets([]v1alpha1.PolicyBundleSet{set("team-a", "policies", "[")}, b.Documents)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid glob")
}

func TestDeriveSetsRejectsSetWithNoClaims(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	_, err = deriveSets([]v1alpha1.PolicyBundleSet{set("team-a", "policies", "nope/**")}, b.Documents)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "claims no indexed document")
}

func TestDeriveSetsRejectsSetWhoseGlobMatchesOnlyWrongKind(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"exceptions/exempt.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: exempt
spec:
  policyRefs:
  - name: require-labels
    kind: ValidatingPolicy
`,
		"policies/require-labels.yaml": validVP,
	})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	_, err = deriveSets([]v1alpha1.PolicyBundleSet{set("team-a", "policies", "exceptions/**")}, b.Documents)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "claims no indexed document")
}

func TestDeriveSetsRejectsSetWhoseGlobMatchesOnlyCommentOnlyFile(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"policy.yaml": validVP,
		"notes.yaml":  "# just a comment\n",
	})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	_, err = deriveSets([]v1alpha1.PolicyBundleSet{set("team-a", "policies", "notes.yaml")}, b.Documents)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "claims no indexed document")
}

func TestDeriveSetsRejectsReservedNameSetWithNoClaimsEvenWithFallbackMembers(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	// "policies" is declared but its glob matches nothing; the resource would fall back to the
	// by-kind default "policies" set in assignSets, but that fallback membership does not
	// exempt the declared set from the no-claim rule.
	_, err = deriveSets([]v1alpha1.PolicyBundleSet{set("policies", "policies", "nope/**")}, b.Documents)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "claims no indexed document")
}

// TestPolicyBundleSpecEveryLeafFieldIsString guards the assumption rejectCoercedScalars depends
// on: PolicyBundleSpec has scalar string fields, a map of strings, a nested object, and a list of
// nested objects, but every leaf scalar at the bottom of all of that, however deep, is a string.
// If a future field is added whose leaf scalars aren't, the gate would reject every legitimate
// value of it with no test failing here to say why; this test is that failure point instead.
func TestPolicyBundleSpecEveryLeafFieldIsString(t *testing.T) {
	assertLeavesAreStrings(t, reflect.TypeOf(v1alpha1.PolicyBundleSpec{}), "PolicyBundleSpec")
}

func assertLeavesAreStrings(t *testing.T, typ reflect.Type, path string) {
	t.Helper()
	switch typ.Kind() {
	case reflect.String:
	case reflect.Ptr, reflect.Slice, reflect.Array:
		assertLeavesAreStrings(t, typ.Elem(), path+"[]")
	case reflect.Map:
		assert.Equal(t, reflect.String, typ.Key().Kind(), "%s: map key must be string", path)
		assertLeavesAreStrings(t, typ.Elem(), path+"[]")
	case reflect.Struct:
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			assertLeavesAreStrings(t, f.Type, path+"."+f.Name)
		}
	default:
		t.Fatalf("%s: field type %s is neither a string nor a slice/map/struct of strings; "+
			"descriptor.go's unquoted-scalar coercion gate assumes every leaf here is a string "+
			"and must be updated to match if this is intentional", path, typ.Kind())
	}
}
