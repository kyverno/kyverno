package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
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

func TestAssembleExcludesDescriptorAtRoot(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"policy.yaml":               validVP,
		internal.DescriptorFilename: "apiVersion: cli.kyverno.io/v1alpha1\nkind: PolicyBundle\nmetadata:\n  name: x\n",
	})
	b, err := Assemble(dir)
	require.NoError(t, err)
	assert.Len(t, b.Documents, 1)
	_, archived := b.Files[internal.DescriptorFilename]
	assert.False(t, archived, "the descriptor must not be archived")
}

func TestAssembleRejectsDescriptorBelowRoot(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"policy.yaml":                           validVP,
		"nested/" + internal.DescriptorFilename: "apiVersion: cli.kyverno.io/v1alpha1\nkind: PolicyBundle\nmetadata:\n  name: x\n",
	})
	_, err := Assemble(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reserved bundle descriptor")
}

func TestAssembleRejectsCliKyvernoIoDocumentAnywhere(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"policy.yaml": validVP,
		"nested/config.yaml": `apiVersion: cli.kyverno.io/v1alpha1
kind: Values
metadata:
  name: x
`,
	})
	_, err := Assemble(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cli.kyverno.io")
}

func TestAssembleRejectsV1List(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"list.yaml": `apiVersion: v1
kind: List
items:
- apiVersion: policies.kyverno.io/v1beta1
  kind: ValidatingPolicy
  metadata:
    name: a
  spec:
    validations:
    - expression: "true"
`,
	})
	_, err := Assemble(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "v1 List")
}

func TestAssembleSkipsHiddenAndNonYAMLFiles(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"policy.yaml":      validVP,
		".hidden.yaml":     "not: valid: yaml: at: all",
		"README.md":        "# not a resource",
		".git/config.yaml": "not: valid: yaml: at: all",
	})
	b, err := Assemble(dir)
	require.NoError(t, err)
	assert.Len(t, b.Documents, 1)
	assert.Equal(t, "policy.yaml", b.Documents[0].Path)
}

func TestAssembleRejectsSymlink(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	require.NoError(t, os.Symlink(filepath.Join(dir, "policy.yaml"), filepath.Join(dir, "link.yaml")))
	_, err := Assemble(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is a symlink")
}

// TestAssembleRejectsSymlinkedDirectoryInTree is the regression test for the round-2 MAJOR
// finding: a symlinked directory whose own name has no .yaml/.yml extension must not be silently
// dropped. Before the fix, WalkDir's DirEntry.IsDir() is false for a symlink (even one pointing
// at a directory), so the entry fell through both the directory branch (never descended into)
// and the extension branch (no recognized extension), and Assemble returned no error while
// silently omitting everything under it.
func TestAssembleRejectsSymlinkedDirectoryInTree(t *testing.T) {
	sharedDir := writeTree(t, map[string]string{"shared.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: shared
spec:
  validations:
  - expression: "true"
`})
	root := writeTree(t, map[string]string{"p.yaml": validVP})
	require.NoError(t, os.Symlink(sharedDir, filepath.Join(root, "common")))

	_, err := Assemble(root)
	assert.Error(t, err, "a symlinked directory in the tree must be rejected, not silently dropped")
	assert.Contains(t, err.Error(), "is a symlink")
	assert.Contains(t, err.Error(), "common")
}

// TestAssembleToleratesHiddenSymlink is the round-3 regression test: a hidden-named symlink
// (whether it points at a file or a directory) must be as invisible as any other hidden path,
// matching old push (its hidden check keys off the given path's own name, not what it resolves
// to) and bundle-spec.md section 5's "not an error" list. Before the fix, the symlink check ran
// before the hidden-name check, so this failed the push for no reason the format ever required.
func TestAssembleToleratesHiddenSymlink(t *testing.T) {
	sharedDir := writeTree(t, map[string]string{"shared.yaml": validVP})
	root := writeTree(t, map[string]string{"p.yaml": validVP})
	require.NoError(t, os.Symlink(sharedDir, filepath.Join(root, ".venv")))

	b, err := Assemble(root)
	require.NoError(t, err, "a hidden symlink must be tolerated like any other hidden path")
	require.Len(t, b.Documents, 1)
	assert.Equal(t, "p.yaml", b.Documents[0].Path)
}

// TestAssembleToleratesNonYAMLSymlinkToRegularFile is TestAssembleToleratesHiddenSymlink's
// sibling for the other content-free case: a non-hidden symlink whose own name has no
// .yaml/.yml extension and that resolves to a regular file (not a directory) carries no bundle
// content either way, so it must be silently excluded rather than rejected, the same as any
// other non-YAML path — matching old push, which read straight through it with no rejection of
// its own.
func TestAssembleToleratesNonYAMLSymlinkToRegularFile(t *testing.T) {
	root := writeTree(t, map[string]string{
		"p.yaml":    validVP,
		"README.md": "# not a resource",
	})
	require.NoError(t, os.Symlink(filepath.Join(root, "README.md"), filepath.Join(root, "README-link.md")))

	b, err := Assemble(root)
	require.NoError(t, err, "a symlink to a regular non-YAML file must be tolerated like any other non-YAML path")
	require.Len(t, b.Documents, 1)
	assert.Equal(t, "p.yaml", b.Documents[0].Path)
}

func TestAssembleResolvesSymlinkedRoot(t *testing.T) {
	realDir := writeTree(t, map[string]string{"p.yaml": validVP})
	linkRoot := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(realDir, linkRoot))

	b, err := Assemble(linkRoot)
	require.NoError(t, err, "a symlinked bundle root must resolve to its target, not report zero resources")
	require.Len(t, b.Documents, 1)
	assert.Equal(t, "p.yaml", b.Documents[0].Path)
}

func TestAssembleArchivesCommentOnlyYAMLFile(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"ok.yaml":    validVP,
		"notes.yaml": "# just a comment\n",
	})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.Len(t, b.Documents, 1, "the comment-only file contributes no resources[] entry")
	assert.Contains(t, b.Files, "ok.yaml")
	assert.Contains(t, b.Files, "notes.yaml", "a comment-only .yaml file must still be archived with its original bytes")
	assert.Equal(t, "# just a comment\n", string(b.Files["notes.yaml"]))
}

func TestAssembleRejectsEmptyInputTree(t *testing.T) {
	dir := t.TempDir()
	_, err := Assemble(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "zero resources")
}

func TestAssembleRejectsInputThatIsOnlyTheDescriptor(t *testing.T) {
	dir := writeTree(t, map[string]string{
		internal.DescriptorFilename: "apiVersion: cli.kyverno.io/v1alpha1\nkind: PolicyBundle\nmetadata:\n  name: x\n",
	})
	_, err := Assemble(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "zero resources")
}

func TestAssembleSingleFileInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	require.NoError(t, os.WriteFile(path, []byte(validVP), 0o600))
	b, err := Assemble(path)
	require.NoError(t, err)
	require.Len(t, b.Documents, 1)
	assert.Equal(t, "policy.yaml", b.Documents[0].Path)
}

// TestAssembleRejectsNonYAMLSingleFileInput is the regression test for the round-2 MAJOR
// finding: `kyverno oci push ./policy.json` must fail rather than silently produce a bundle that
// skipped CEL compilation. Before the fix, the single-file branch had no extension gate:
// assembleFile's own raw-document scan happily parses JSON (a YAML subset) and indexes the
// policy, while policy.Load's fsLoad silently returns an empty, error-free LoaderResults for a
// non-.yaml/.yml path — so Validate had nothing to CEL-compile and passed a policy with a
// syntactically invalid expression straight through to Write.
func TestAssembleRejectsNonYAMLSingleFileInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
  "apiVersion": "policies.kyverno.io/v1beta1",
  "kind": "ValidatingPolicy",
  "metadata": {"name": "bad-policy"},
  "spec": {"validations": [{"expression": "this is not (( valid cel"}]}
}`), 0o600))

	_, err := Assemble(path)
	assert.Error(t, err, "a .json single-file input must be rejected, not silently bundled unvalidated")
	assert.Contains(t, err.Error(), ".json")
}

// TestAssembleRejectsHiddenSingleFileInput is TestAssembleRejectsNonYAMLSingleFileInput's
// sibling for the other cause of the same gap: a hidden filename, which policy.Load's fsLoad
// also silently skips with no error.
func TestAssembleRejectsHiddenSingleFileInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".policy.yaml")
	require.NoError(t, os.WriteFile(path, []byte(validVP), 0o600))

	_, err := Assemble(path)
	assert.Error(t, err, "a hidden single-file input must be rejected, not silently bundled unvalidated")
	assert.Contains(t, err.Error(), "hidden")
}

func TestWriteContentLayerIsDeterministicTar(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"b.yaml": validVP,
		"a.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: a-policy
spec:
  validations:
  - expression: "true"
`,
	})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))
	img, err := Write(b, nil)
	require.NoError(t, err)

	layers, err := img.Layers()
	require.NoError(t, err)
	require.Len(t, layers, 1)

	rc, err := layers[0].Compressed()
	require.NoError(t, err)
	defer rc.Close()
	gz, err := gzip.NewReader(rc)
	require.NoError(t, err)
	assert.True(t, gz.ModTime.IsZero(), "gzip header mtime must be zero")
	assert.Equal(t, "", gz.Name, "gzip header must not store a filename")

	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		names = append(names, hdr.Name)
		assert.Equal(t, int64(0), hdr.ModTime.Unix(), "entry mtime must be the Unix epoch")
		assert.Equal(t, int64(0o644), hdr.Mode, "entry mode must be 0644")
		assert.Equal(t, 0, hdr.Uid)
		assert.Equal(t, 0, hdr.Gid)
		assert.Equal(t, "", hdr.Uname)
		assert.Equal(t, "", hdr.Gname)
		assert.Equal(t, byte(tar.TypeReg), hdr.Typeflag)
	}
	assert.Equal(t, []string{"a.yaml", "b.yaml"}, names, "entries must be sorted by path")
}

func TestWriteConfigDescriptorSetsAgreeWithManifestConfig(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"policy.yaml": validVP,
		"exception.yaml": `apiVersion: policies.kyverno.io/v1beta1
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
	img, err := Write(b, nil)
	require.NoError(t, err)

	manifest, err := img.Manifest()
	require.NoError(t, err)

	// Verifies the wrapper actually rewires the manifest's config descriptor: it must point at
	// our config bytes, not at an empty.Image {} config.
	configBytes, err := img.RawConfigFile()
	require.NoError(t, err)
	assert.Equal(t, int64(len(configBytes)), manifest.Config.Size)
	assert.Equal(t, string(internal.ConfigMediaType), string(manifest.Config.MediaType))

	var cfg Config
	require.NoError(t, json.Unmarshal(configBytes, &cfg))
	assert.Equal(t, "1.0.0", cfg.FormatVersion)
	require.Len(t, cfg.Sets, 2)
	require.Len(t, cfg.Resources, 2)
	// No descriptor: name, version, description, kyvernoVersion, source, and created are
	// omitted, and so are their matching manifest annotations.
	assert.Empty(t, cfg.Name)
	assert.Empty(t, cfg.Version)
	assert.Empty(t, cfg.Created)
	assert.NotContains(t, manifest.Annotations, internal.AnnotationName)
	assert.NotContains(t, manifest.Annotations, internal.AnnotationVersion)
	assert.NotContains(t, manifest.Annotations, internal.AnnotationOCICreated)
}

// TestWriteStripsReservedAnnotationsFromDescriptorEvenWhenConfigLeavesThemEmpty is the round-3
// regression test: a descriptor that sets a reserved io.kyverno.bundle.* or
// org.opencontainers.image.* annotation directly, for a field the config itself leaves empty
// (no d.Name, no d.Created, ...), must not have that value survive into the manifest. Before the
// fix, buildAnnotations only ever *overwrote* a reserved key when the config populated the
// matching field, so a reserved key with nothing to overwrite it kept the descriptor's — meaning
// a forged io.kyverno.bundle.version, org.opencontainers.image.title, or
// org.opencontainers.image.created reached the manifest untouched.
func TestWriteStripsReservedAnnotationsFromDescriptorEvenWhenConfigLeavesThemEmpty(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	d := &Descriptor{
		// Name, Version, and Created are deliberately left unset, so the config carries no
		// value for any of these three fields.
		Annotations: map[string]string{
			internal.AnnotationVersion:    "9.9.9",
			internal.AnnotationOCITitle:   "spoofed",
			internal.AnnotationOCICreated: "2020-01-01T00:00:00Z",
			// A non-reserved key must still pass through untouched.
			"example.com/team": "team-a",
		},
	}
	img, err := Write(b, d)
	require.NoError(t, err)

	manifest, err := img.Manifest()
	require.NoError(t, err)

	assert.NotContains(t, manifest.Annotations, internal.AnnotationVersion, "a descriptor must not forge io.kyverno.bundle.version when the config has no version")
	assert.NotContains(t, manifest.Annotations, internal.AnnotationOCITitle, "a descriptor must not forge org.opencontainers.image.title when the config has no name")
	assert.NotContains(t, manifest.Annotations, internal.AnnotationOCICreated, "a descriptor must not forge org.opencontainers.image.created when the config has no created")
	assert.Equal(t, "team-a", manifest.Annotations["example.com/team"], "a non-reserved descriptor annotation must still pass through")
}

func TestWriteOmitsEmptySet(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))
	img, err := Write(b, nil)
	require.NoError(t, err)

	configBytes, err := img.RawConfigFile()
	require.NoError(t, err)
	var cfg Config
	require.NoError(t, json.Unmarshal(configBytes, &cfg))
	require.Len(t, cfg.Sets, 1, "a policies-only bundle must emit one set, not two")
	assert.Equal(t, "policies", cfg.Sets[0].Name)
}

// TestAssignSetsDescriptorOverrideFallsBackForUnassignedResources covers a descriptor whose Sets
// don't name every resource in the bundle: every resources[] entry must still get a non-empty
// `set` that names an entry in sets[] (bundle-spec.md section 6), by falling back to the by-kind
// default for anything the descriptor doesn't cover.
func TestAssignSetsDescriptorOverrideFallsBackForUnassignedResources(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"a.yaml": validVP,
		"b.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: b-policy
spec:
  validations:
  - expression: "true"
`,
	})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))

	// Only "require-labels" (a.yaml) is named by the descriptor; "b-policy" (b.yaml) is not.
	d := &Descriptor{
		Sets: []SetAssignment{
			{Kind: "ValidatingPolicy", Name: "require-labels", Set: "custom", Type: "policies"},
		},
	}
	img, err := Write(b, d)
	require.NoError(t, err)

	configBytes, err := img.RawConfigFile()
	require.NoError(t, err)
	var cfg Config
	require.NoError(t, json.Unmarshal(configBytes, &cfg))

	setNames := map[string]bool{}
	for _, s := range cfg.Sets {
		setNames[s.Name] = true
	}
	for _, r := range cfg.Resources {
		assert.NotEmpty(t, r.Set, "%s must have a non-empty set", r.Name)
		assert.True(t, setNames[r.Set], "%s's set %q must name an entry in sets[]", r.Name, r.Set)
	}
	// The overridden resource got the descriptor's set; the uncovered one fell back to default.
	byName := map[string]string{}
	for _, r := range cfg.Resources {
		byName[r.Name] = r.Set
	}
	assert.Equal(t, "custom", byName["require-labels"])
	assert.Equal(t, "policies", byName["b-policy"])
}

func TestWriteHonorsSourceDateEpoch(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "1700000000")
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))
	img, err := Write(b, nil)
	require.NoError(t, err)

	configBytes, err := img.RawConfigFile()
	require.NoError(t, err)
	var cfg Config
	require.NoError(t, json.Unmarshal(configBytes, &cfg))
	assert.Equal(t, "2023-11-14T22:13:20Z", cfg.Created)

	manifest, err := img.Manifest()
	require.NoError(t, err)
	assert.Equal(t, "2023-11-14T22:13:20Z", manifest.Annotations[internal.AnnotationOCICreated])
}

func TestWriteRejectsUnparseableSourceDateEpoch(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "not-a-number")
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))
	_, err = Write(b, nil)
	assert.Error(t, err)
}

func TestReadCrossChecksIndexAgainstArchive(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))
	img, err := Write(b, nil)
	require.NoError(t, err)

	dst := t.TempDir()
	got, err := Read(img, dst)
	require.NoError(t, err)
	require.Len(t, got.Documents, 1)
}

// TestReadRejectsUnsupportedFormatMajorVersion is the regression test for reader MUST 6: a
// bundle whose config formatVersion is a major version this reader doesn't support must be
// rejected, naming both versions, rather than silently accepted because Read never looked at the
// field.
func TestReadRejectsUnsupportedFormatMajorVersion(t *testing.T) {
	dir := writeTree(t, map[string]string{"p.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))
	img, err := Write(b, nil)
	require.NoError(t, err)

	configBytes, err := img.RawConfigFile()
	require.NoError(t, err)
	var cfg Config
	require.NoError(t, json.Unmarshal(configBytes, &cfg))
	cfg.FormatVersion = "2.0.0"
	tamperedConfig, err := json.Marshal(cfg)
	require.NoError(t, err)

	layers, err := img.Layers()
	require.NoError(t, err)
	tamperedImg, err := newImage(layers[0], tamperedConfig, nil)
	require.NoError(t, err)

	_, err = Read(tamperedImg, t.TempDir())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported bundle format major version 2")
	assert.Contains(t, err.Error(), "this reader supports major version 1")
}

// TestCheckFormatVersion pins the malformed-vs-unsupported-major distinction: checkFormatVersion
// requires a full three-component numeric semver string before it ever compares the major
// version, so a truncated or non-numeric formatVersion is rejected as malformed rather than
// silently accepted as some major version prefix.
func TestCheckFormatVersion(t *testing.T) {
	tests := []struct {
		name          string
		formatVersion string
		wantErr       string
	}{
		{name: "valid", formatVersion: "1.0.0", wantErr: ""},
		{name: "unsupported major", formatVersion: "2.0.0", wantErr: "unsupported bundle format major version 2"},
		{name: "truncated at first dot", formatVersion: "1.bad", wantErr: "not a valid semver string"},
		{name: "trailing dot", formatVersion: "1.", wantErr: "not a valid semver string"},
		{name: "two components", formatVersion: "1.0", wantErr: "not a valid semver string"},
		{name: "one component", formatVersion: "1", wantErr: "not a valid semver string"},
		{name: "empty", formatVersion: "", wantErr: "not a valid semver string"},
		{name: "four components", formatVersion: "1.0.0.0", wantErr: "not a valid semver string"},
		{name: "non-numeric", formatVersion: "a.b.c", wantErr: "not a valid semver string"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkFormatVersion(tc.formatVersion)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// TestPlaceFailsAllOrNothingWhenATargetIsBlocked is the regression test for the MINOR finding
// that place left partial output behind on a mid-way failure: before the fix, a.yaml (processed
// before the colliding b.yaml in map iteration order, on at least some runs) was left on disk
// even though the overall Read call failed.
func TestPlaceFailsAllOrNothingWhenATargetIsBlocked(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"a.yaml": validVP,
		"b.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: b-policy
spec:
  validations:
  - expression: "true"
`,
	})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))
	img, err := Write(b, nil)
	require.NoError(t, err)

	dst := t.TempDir()
	// b.yaml already exists as a directory in the target: place must refuse to write it, and
	// must not have already written a.yaml before discovering that.
	require.NoError(t, os.MkdirAll(filepath.Join(dst, "b.yaml"), 0o750))

	_, err = Read(img, dst)
	assert.Error(t, err)

	_, statErr := os.Stat(filepath.Join(dst, "a.yaml"))
	assert.True(t, os.IsNotExist(statErr), "a.yaml must not be left behind when the overall placement failed")
}

// TestPlaceFailsAllOrNothingWhenAnIntermediateComponentIsARegularFile is the round-3 regression
// test: a regular (non-directory, non-symlink) file sitting at an intermediate path component
// used to pass the pre-flight check entirely (it's neither a symlink nor the leaf, so neither
// branch fired), and the failure only surfaced later, mid-write, as an ENOTDIR from MkdirAll —
// by which point files ordered before the blocked one in map iteration were already on disk.
func TestPlaceFailsAllOrNothingWhenAnIntermediateComponentIsARegularFile(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"a.yaml": validVP,
		"policies/b.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: b-policy
spec:
  validations:
  - expression: "true"
`,
	})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))
	img, err := Write(b, nil)
	require.NoError(t, err)

	dst := t.TempDir()
	// "policies" already exists as a regular file, not a directory: the bundle wants to write
	// policies/b.yaml under it, which is impossible.
	require.NoError(t, os.WriteFile(filepath.Join(dst, "policies"), []byte("not a directory"), 0o600))

	_, err = Read(img, dst)
	assert.Error(t, err)

	_, statErr := os.Stat(filepath.Join(dst, "a.yaml"))
	assert.True(t, os.IsNotExist(statErr), "a.yaml must not be left behind when the overall placement failed")
}

// TestPlaceRefusesToWriteThroughSymlinkedDirectory is the regression test for the MINOR finding
// that a pre-existing symlinked directory in the target got silently written through:
// securejoin's confinement relocated the write to a mangled, nested path under dst while Read
// still reported success, so the file never ended up where its path said and nothing told the
// caller.
func TestPlaceRefusesToWriteThroughSymlinkedDirectory(t *testing.T) {
	dir := writeTree(t, map[string]string{"policies/a.yaml": validVP})
	b, err := Assemble(dir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))
	img, err := Write(b, nil)
	require.NoError(t, err)

	dst := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(dst, "policies")))

	_, err = Read(img, dst)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "symlink")

	_, statErr := os.Stat(filepath.Join(outside, "a.yaml"))
	assert.True(t, os.IsNotExist(statErr), "the file must not have been written through the symlink")
}

// TestReadNeverPlacesUnindexedArchiveContent is the regression test for the MAJOR finding: a
// tampered archive that adds a root kyverno-bundle.yaml (holding a smuggled ValidatingPolicy,
// never CEL-compiled, never identity-checked, never indexed) and a non-YAML evil.sh alongside a
// legitimately indexed p.yaml must not let either extra file reach disk, and must not let the
// smuggled policy surface to a caller (e.g. `kyverno apply oci://`, `kyverno test oci://`) that
// loads the directory Read wrote to. Before the fix, Read returned no error, the on-disk tree
// was [evil.sh kyverno-bundle.yaml p.yaml], and policy.Load(dst) returned both "smuggled" and
// "require-labels" with no error: a policy that had passed no validation was loaded straight
// into the caller.
func TestReadNeverPlacesUnindexedArchiveContent(t *testing.T) {
	legitDir := writeTree(t, map[string]string{"p.yaml": validVP})
	b, err := Assemble(legitDir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))
	legitImg, err := Write(b, nil)
	require.NoError(t, err)
	configBytes, err := legitImg.RawConfigFile()
	require.NoError(t, err)

	tamperedFiles := map[string][]byte{
		"p.yaml": b.Files["p.yaml"],
		internal.DescriptorFilename: []byte(`apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: smuggled
spec:
  validations:
  - expression: "true"
`),
		"evil.sh": []byte("#!/bin/sh\necho pwned\n"),
	}
	tamperedLayer, err := buildContentLayer(tamperedFiles)
	require.NoError(t, err)
	// The config is untouched: it still indexes only p.yaml. A conformant writer would never
	// produce this combination; this simulates a tampered or hand-built malicious image.
	tamperedImg, err := newImage(tamperedLayer, configBytes, map[string]string{internal.AnnotationFormatVersion: internal.FormatVersion})
	require.NoError(t, err)

	dst := t.TempDir()
	_, err = Read(tamperedImg, dst)
	require.NoError(t, err, "extra excluded-class content alongside valid, indexed content is not itself an error")

	_, statErr := os.Stat(filepath.Join(dst, internal.DescriptorFilename))
	assert.True(t, os.IsNotExist(statErr), "the smuggled descriptor must never reach disk")
	_, statErr = os.Stat(filepath.Join(dst, "evil.sh"))
	assert.True(t, os.IsNotExist(statErr), "the non-YAML file must never reach disk")

	loaded, err := policy.Load(nil, "", false, dst)
	require.NoError(t, err)
	var names []string
	for _, vp := range loaded.ValidatingPolicies {
		names = append(names, vp.GetName())
	}
	assert.NotContains(t, names, "smuggled", "a caller loading the extracted directory must never see the smuggled policy")
	assert.Contains(t, names, "require-labels")
}

// TestReadRejectsBelowRootDescriptorInArchive covers the harder tamper variant: a reserved
// descriptor filename below the bundle root is a hard write-side error (assemble.go), so a
// tampered archive carrying one must fail Read outright rather than merely excluding it.
func TestReadRejectsBelowRootDescriptorInArchive(t *testing.T) {
	legitDir := writeTree(t, map[string]string{"p.yaml": validVP})
	b, err := Assemble(legitDir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))
	legitImg, err := Write(b, nil)
	require.NoError(t, err)
	configBytes, err := legitImg.RawConfigFile()
	require.NoError(t, err)

	tamperedFiles := map[string][]byte{
		"p.yaml": b.Files["p.yaml"],
		"nested/" + internal.DescriptorFilename: []byte(`apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: smuggled
spec:
  validations:
  - expression: "true"
`),
	}
	tamperedLayer, err := buildContentLayer(tamperedFiles)
	require.NoError(t, err)
	tamperedImg, err := newImage(tamperedLayer, configBytes, nil)
	require.NoError(t, err)

	dst := t.TempDir()
	_, err = Read(tamperedImg, dst)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reserved bundle descriptor")
}

// TestReadDoesNotTreatPreExistingFilesAsBundleContent is the regression test for the second
// MAJOR finding: Read must validate and index only the archive's own entries, never whatever
// already happens to be sitting in the target directory. Before the fix, Read ran Assemble
// directly over dir, so a pre-existing, unrelated YAML file in dir was treated as bundle content
// and failed the cross-check with "no matching config index entry" even though the bundle itself
// was perfectly valid.
// TestReadDoesNotLeakStagingPathIntoErrors is the round-3 regression test: when Assemble fails
// while Read is re-validating the extracted, private staging directory, the resulting error must
// not contain that directory's path. An empty content layer (an archive with no files at all, as
// a tampered writer might produce) is what drives Assemble's own "yields zero resources"
// rejection, which is the case the finding observed leaking a temp path like
// /var/folders/.../kyverno-oci-read-123.
func TestReadDoesNotLeakStagingPathIntoErrors(t *testing.T) {
	emptyLayer, err := buildContentLayer(map[string][]byte{})
	require.NoError(t, err)
	configBytes, err := json.Marshal(Config{FormatVersion: internal.FormatVersion})
	require.NoError(t, err)
	img, err := newImage(emptyLayer, configBytes, nil)
	require.NoError(t, err)

	_, err = Read(img, t.TempDir())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "yields zero resources")
	assert.NotContains(t, err.Error(), os.TempDir(), "the error must not leak Read's private staging directory")
	assert.NotContains(t, err.Error(), "kyverno-oci-read-", "the error must not leak Read's staging directory name")
}

func TestReadDoesNotTreatPreExistingFilesAsBundleContent(t *testing.T) {
	srcDir := writeTree(t, map[string]string{"p.yaml": validVP})
	b, err := Assemble(srcDir)
	require.NoError(t, err)
	require.NoError(t, Validate(b))
	img, err := Write(b, nil)
	require.NoError(t, err)

	dst := t.TempDir()
	// A file that was never part of any bundle, already present in the target directory.
	require.NoError(t, os.WriteFile(filepath.Join(dst, "other.yaml"), []byte(`apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: pre-existing
spec:
  validations:
  - expression: "true"
`), 0o600))

	_, err = Read(img, dst)
	require.NoError(t, err, "a pre-existing, unrelated file in the target directory must not fail the pull")

	// Both files are present afterward: the bundle's own content was placed, and the
	// pre-existing file was left alone.
	_, err = os.Stat(filepath.Join(dst, "p.yaml"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(dst, "other.yaml"))
	assert.NoError(t, err)
}

// TestReadIntoDirectoryThatAlreadyReceivedAnotherBundle covers the second scenario the MAJOR
// finding named explicitly: pulling bundle B into a directory that already received bundle A.
func TestReadIntoDirectoryThatAlreadyReceivedAnotherBundle(t *testing.T) {
	dst := t.TempDir()

	bA, err := Assemble(writeTree(t, map[string]string{"a.yaml": validVP}))
	require.NoError(t, err)
	require.NoError(t, Validate(bA))
	imgA, err := Write(bA, nil)
	require.NoError(t, err)
	_, err = Read(imgA, dst)
	require.NoError(t, err)

	bB, err := Assemble(writeTree(t, map[string]string{"b.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: b-policy
spec:
  validations:
  - expression: "true"
`}))
	require.NoError(t, err)
	require.NoError(t, Validate(bB))
	imgB, err := Write(bB, nil)
	require.NoError(t, err)

	_, err = Read(imgB, dst)
	require.NoError(t, err, "pulling a second bundle into a directory that already received one must not fail")

	_, err = os.Stat(filepath.Join(dst, "a.yaml"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(dst, "b.yaml"))
	assert.NoError(t, err)
}

// defaultSets is a minimal, valid sets[] for tests that don't otherwise care about set
// assignment: every ResourceEntry below names "policies", so this is what a call to
// crossCheckIndex needs to pass the set-membership check and reach the check under test.
var defaultSets = []ConfigSet{{Name: "policies", Type: "policies"}}

func TestCrossCheckIndexRejectsMismatchedDigest(t *testing.T) {
	documents := []Document{{Path: "a.yaml", Index: 0, Digest: "sha256:aaa"}}
	resources := []ResourceEntry{{Path: "a.yaml", DocumentIndex: 0, Digest: "sha256:bbb", Set: "policies"}}
	err := crossCheckIndex(documents, resources, defaultSets)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestCrossCheckIndexRejectsUnmatchedArchiveDocument(t *testing.T) {
	documents := []Document{{Path: "a.yaml", Index: 0, Digest: "sha256:aaa"}}
	err := crossCheckIndex(documents, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no matching config index entry")
}

func TestCrossCheckIndexRejectsUnmatchedIndexEntry(t *testing.T) {
	resources := []ResourceEntry{{Path: "a.yaml", DocumentIndex: 0, Digest: "sha256:aaa", Set: "policies"}}
	err := crossCheckIndex(nil, resources, defaultSets)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no matching archived document")
}

// TestCrossCheckIndexRejectsUnknownSet pins the set-membership half of reader MUST 4: an index
// entry naming a set that isn't in the config's own sets[] breaks the index contract just as
// badly as a mismatched identity field would, even when its path, documentIndex, digest, and
// identity all otherwise match an archived document.
func TestCrossCheckIndexRejectsUnknownSet(t *testing.T) {
	documents := []Document{{Path: "a.yaml", Index: 0, Digest: "sha256:aaa"}}

	t.Run("unknown set", func(t *testing.T) {
		resources := []ResourceEntry{{Path: "a.yaml", DocumentIndex: 0, Digest: "sha256:aaa", Set: "ghost"}}
		err := crossCheckIndex(documents, resources, defaultSets)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ghost")
		assert.Contains(t, err.Error(), "not in the config's sets[]")
	})

	t.Run("empty set", func(t *testing.T) {
		resources := []ResourceEntry{{Path: "a.yaml", DocumentIndex: 0, Digest: "sha256:aaa", Set: ""}}
		err := crossCheckIndex(documents, resources, defaultSets)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not in the config's sets[]")
	})
}

// TestCrossCheckIndexRejectsMismatchedIdentity pins reader MUST 4's identity check: a config
// index entry that matches an archived document by path, documentIndex, and digest but disagrees
// on apiVersion, kind, namespace, or name must still be rejected, naming the field that
// disagreed. Checking only the digest would let an index entry lie about a document's identity
// to any consumer that trusts the index (for example, `kyverno oci inspect`) without re-deriving
// the fields from the archive itself.
func TestCrossCheckIndexRejectsMismatchedIdentity(t *testing.T) {
	base := Document{
		Path: "a.yaml", Index: 0, Digest: "sha256:aaa",
		APIVersion: "policies.kyverno.io/v1beta1", Kind: "ValidatingPolicy", Namespace: "", Name: "require-labels",
	}
	baseEntry := ResourceEntry{
		Path: "a.yaml", DocumentIndex: 0, Digest: "sha256:aaa", Set: "policies",
		APIVersion: "policies.kyverno.io/v1beta1", Kind: "ValidatingPolicy", Namespace: "", Name: "require-labels",
	}

	tests := []struct {
		name    string
		mutate  func(e ResourceEntry) ResourceEntry
		wantMsg string
	}{
		{
			name:    "apiVersion",
			mutate:  func(e ResourceEntry) ResourceEntry { e.APIVersion = "policies.kyverno.io/v2beta1"; return e },
			wantMsg: "apiVersion",
		},
		{
			name:    "kind",
			mutate:  func(e ResourceEntry) ResourceEntry { e.Kind = "NamespacedValidatingPolicy"; return e },
			wantMsg: "kind",
		},
		{
			name:    "namespace",
			mutate:  func(e ResourceEntry) ResourceEntry { e.Namespace = "team-a"; return e },
			wantMsg: "namespace",
		},
		{
			name:    "name",
			mutate:  func(e ResourceEntry) ResourceEntry { e.Name = "other-name"; return e },
			wantMsg: "name",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := crossCheckIndex([]Document{base}, []ResourceEntry{tc.mutate(baseEntry)}, defaultSets)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantMsg)
			assert.Contains(t, err.Error(), "does not match the archived document")
		})
	}
}

func TestCheckManifestShapeDetectsLegacyImage(t *testing.T) {
	manifest := &v1.Manifest{
		Config: v1.Descriptor{MediaType: types.MediaType(internal.LegacyConfigMediaType)},
		Layers: []v1.Descriptor{{MediaType: types.MediaType(internal.LegacyContentMediaType)}},
	}
	err := checkManifestShape(manifest)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "legacy image format")
}

func TestCheckManifestShapeRejectsMultipleLayers(t *testing.T) {
	manifest := &v1.Manifest{
		Config: v1.Descriptor{MediaType: types.MediaType(internal.ConfigMediaType)},
		Layers: []v1.Descriptor{
			{MediaType: types.MediaType(internal.ContentMediaType)},
			{MediaType: types.MediaType(internal.ContentMediaType)},
		},
	}
	err := checkManifestShape(manifest)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one layer")
}

func validManifest() *v1.Manifest {
	return &v1.Manifest{
		SchemaVersion: 2,
		MediaType:     types.OCIManifestSchema1,
		Config:        v1.Descriptor{MediaType: types.MediaType(internal.ConfigMediaType)},
		Layers:        []v1.Descriptor{{MediaType: types.MediaType(internal.ContentMediaType)}},
	}
}

func TestCheckManifestShapeAcceptsOCIManifest(t *testing.T) {
	assert.NoError(t, checkManifestShape(validManifest()))
}

// TestCheckManifestShapeRejectsNonOCIManifest pins reader MUST 1's schemaVersion/mediaType check:
// bundle-spec.md section 4 requires a plain OCI 1.0 image manifest, so a manifest carrying the
// expected config and layer media types but a Docker schema-2 (or otherwise non-OCI) manifest
// media type must still be rejected, not silently accepted.
func TestCheckManifestShapeRejectsNonOCIManifest(t *testing.T) {
	manifest := validManifest()
	manifest.MediaType = types.DockerManifestSchema2
	err := checkManifestShape(manifest)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OCI 1.0 image manifest")
}

// TestExtractRejectsEntryOverMaxSize pins the per-entry decompression bound: a single entry
// whose declared tar size exceeds maxEntryBytes must be rejected before its content is read into
// memory, naming the limit and the offending entry.
func TestExtractRejectsEntryOverMaxSize(t *testing.T) {
	oversized := bytes.Repeat([]byte("a"), maxEntryBytes+1)
	layer, err := buildContentLayer(map[string][]byte{"big.yaml": oversized})
	require.NoError(t, err)

	err = extract(layer, t.TempDir())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("%d", maxEntryBytes))
	assert.Contains(t, err.Error(), "big.yaml")
}

// TestExtractRejectsTotalOverMaxSize pins the whole-layer decompression bound: several entries
// individually under maxEntryBytes but summing past maxTotalBytes must still be rejected, even
// though no single entry trips the per-entry check.
func TestExtractRejectsTotalOverMaxSize(t *testing.T) {
	perEntry := maxEntryBytes // exactly at the per-entry limit, so only the running total trips.
	files := make(map[string][]byte)
	entriesNeeded := maxTotalBytes/perEntry + 1
	for i := 0; i < entriesNeeded; i++ {
		files[fmt.Sprintf("f%03d.yaml", i)] = bytes.Repeat([]byte("a"), perEntry)
	}
	layer, err := buildContentLayer(files)
	require.NoError(t, err)

	err = extract(layer, t.TempDir())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("%d", maxTotalBytes))
	assert.Contains(t, err.Error(), "total decompressed size")
}

// TestExtractRejectsTooManyEntries pins the entry-count bound: a layer with more than maxEntries
// entries must be rejected regardless of how small each entry is.
func TestExtractRejectsTooManyEntries(t *testing.T) {
	files := make(map[string][]byte, maxEntries+1)
	for i := 0; i < maxEntries+1; i++ {
		files[fmt.Sprintf("f%05d.yaml", i)] = []byte("x")
	}
	layer, err := buildContentLayer(files)
	require.NoError(t, err)

	err = extract(layer, t.TempDir())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("%d", maxEntries))
	assert.Contains(t, err.Error(), "entries")
}
