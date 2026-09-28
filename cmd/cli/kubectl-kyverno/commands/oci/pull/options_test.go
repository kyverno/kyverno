package pull

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal/bundle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTree writes files (bundle-root-relative path -> content) under a fresh temp directory
// and returns its path.
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

// pushAndPull runs the source tree through Assemble, Validate, and Write, then Read, without
// touching a real registry, and returns the directory Read extracted into.
func pushAndPull(t *testing.T, srcDir string) string {
	t.Helper()
	b, err := bundle.Assemble(srcDir)
	require.NoError(t, err)
	require.NoError(t, bundle.Validate(b))
	img, err := bundle.Write(b, nil)
	require.NoError(t, err)

	dstDir := t.TempDir()
	_, err = bundle.Read(img, dstDir)
	require.NoError(t, err)
	return dstDir
}

// buildUnvalidatedImage assembles srcDir and writes it to an image without calling Validate,
// simulating a bundle a non-conformant or malicious writer produced (Write itself never
// validates; the caller does). This is what lets the TestReadRejects* tests below exercise
// Read's own re-validation (reader MUST 3) directly instead of only bundle.Validate — Read must
// reject bad content on its own, not merely rely on push having already refused to produce it.
func buildUnvalidatedImage(t *testing.T, srcDir string) v1.Image {
	t.Helper()
	b, err := bundle.Assemble(srcDir)
	require.NoError(t, err)
	img, err := bundle.Write(b, nil)
	require.NoError(t, err)
	return img
}

func TestRoundTripPreservesNestedLayoutAndBytes(t *testing.T) {
	files := map[string]string{
		"policies/require-labels.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: require-labels
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      resources: ["pods"]
      operations: ["CREATE", "UPDATE"]
  validations:
  - expression: "object.metadata.labels != null"
    message: "labels are required"
`,
		"exceptions/team-a/check-pod.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: check-pod
  namespace: team-a
spec:
  policyRefs:
  - name: require-labels
    kind: ValidatingPolicy
`,
	}
	srcDir := writeTree(t, files)
	dstDir := pushAndPull(t, srcDir)

	for rel, content := range files {
		got, err := os.ReadFile(filepath.Join(dstDir, filepath.FromSlash(rel)))
		require.NoError(t, err, "expected %s to be extracted", rel)
		assert.Equal(t, content, string(got), "%s should round-trip byte-for-byte", rel)
	}
}

func TestRoundTripDeterministicContentDigest(t *testing.T) {
	// Three files across two directories, chosen so a per-directory (filepath.WalkDir) visit
	// order disagrees with a full-path lexical sort: "foo-bar.yaml" sorts before "foo/" as a
	// full path ('-' 0x2D < '/' 0x2F), but a directory walk visits the "foo" subtree, whose
	// entry name "foo" alone sorts before the sibling file "foo-bar.yaml", before it. A writer
	// that forgets the final full-path sort and archives in walk order would still be
	// self-consistent (same wrong order every run) but would disagree with a byte-for-byte
	// comparison against what the tar spec requires; asserting the exact tar entry order below,
	// not just digest equality, is what catches that class of bug.
	files := map[string]string{
		"policies/require-labels.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: require-labels
spec:
  validations:
  - expression: "true"
`,
		"foo-bar.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: foo-bar
spec:
  validations:
  - expression: "true"
`,
		"foo/x.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: foo-x
spec:
  validations:
  - expression: "true"
`,
	}
	wantOrder := []string{"foo-bar.yaml", "foo/x.yaml", "policies/require-labels.yaml"}

	build := func() (string, []string) {
		src := writeTree(t, files)
		b, err := bundle.Assemble(src)
		require.NoError(t, err)
		require.NoError(t, bundle.Validate(b))
		img, err := bundle.Write(b, nil)
		require.NoError(t, err)
		layers, err := img.Layers()
		require.NoError(t, err)
		require.Len(t, layers, 1)
		d, err := layers[0].Digest()
		require.NoError(t, err)

		rc, err := layers[0].Compressed()
		require.NoError(t, err)
		defer rc.Close()
		gz, err := gzip.NewReader(rc)
		require.NoError(t, err)
		tr := tar.NewReader(gz)
		var names []string
		for {
			hdr, err := tr.Next()
			if err != nil {
				break
			}
			names = append(names, hdr.Name)
		}
		return d.String(), names
	}

	digest1, names1 := build()
	digest2, names2 := build()

	assert.Equal(t, wantOrder, names1, "content-layer entries must be sorted by full path, not by directory-walk order")
	assert.Equal(t, names1, names2)
	assert.Equal(t, digest1, digest2, "the same writer against the same input tree must produce the same content-layer digest")
}

// TestAssembleRejectsLegacyKind and TestAssembleRejectsUnknownResources assert Assemble's own
// rejection, not Read's: Assemble's per-file policy.Load call fatally rejects a legacy
// kyverno.io kind (the migration-block predates this change) and any other kind the loader's
// schema recognizes but this package doesn't handle (e.g. a core v1 ConfigMap), before Write is
// ever reached. A bundle carrying either therefore can never be produced in the first place, so
// there is no image to hand to Read for these two cases — unlike TestReadRejectsUnsupportedAPIVersion
// and friends below, which construct an (unvalidated) image and call Read directly, because
// their inputs load without a fatal Assemble error and so can reach Read.
func TestAssembleRejectsLegacyKind(t *testing.T) {
	srcDir := writeTree(t, map[string]string{
		"policy.yaml": `apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: disallow-root
spec:
  rules:
  - name: check-root
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Root user is disallowed."
`,
	})
	_, err := bundle.Assemble(srcDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v1")
}

func TestAssembleRejectsUnknownResources(t *testing.T) {
	srcDir := writeTree(t, map[string]string{
		"config.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  key: value
`,
	})
	_, err := bundle.Assemble(srcDir)
	assert.Error(t, err)
}

func TestReadRejectsUnsupportedAPIVersion(t *testing.T) {
	srcDir := writeTree(t, map[string]string{
		"policy.yaml": `apiVersion: policies.kyverno.io/v1alpha1
kind: ValidatingPolicy
metadata:
  name: alpha-policy
spec:
  validations:
  - expression: "true"
`,
	})
	img := buildUnvalidatedImage(t, srcDir)
	_, err := bundle.Read(img, t.TempDir())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported resource")
	assert.Contains(t, err.Error(), "policies.kyverno.io/v1alpha1")
}

func TestReadRejectsVAP(t *testing.T) {
	// ValidatingAdmissionPolicy is a native k8s type in admissionregistration.k8s.io
	// and must be rejected even though it is a CEL-based type.
	srcDir := writeTree(t, map[string]string{
		"vap.yaml": `apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: check-labels
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      resources: ["pods"]
      operations: ["CREATE"]
  validations:
  - expression: "object.metadata.labels != null"
`,
	})
	img := buildUnvalidatedImage(t, srcDir)
	_, err := bundle.Read(img, t.TempDir())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "native Kubernetes admission policy")
}

func TestReadRejectsPartialIdentity(t *testing.T) {
	// Missing metadata.name.
	srcDir := writeTree(t, map[string]string{
		"policy.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
spec:
  validations:
  - expression: "true"
`,
	})
	img := buildUnvalidatedImage(t, srcDir)
	_, err := bundle.Read(img, t.TempDir())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resource missing kind or metadata.name")
}

func TestReadRejectsUnknownKind(t *testing.T) {
	srcDir := writeTree(t, map[string]string{
		"policy.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: FooBarPolicy
metadata:
  name: unknown-kind
spec:
  validations:
  - expression: "true"
`,
	})
	img := buildUnvalidatedImage(t, srcDir)
	_, err := bundle.Read(img, t.TempDir())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported resource")
}

func TestReadPreservesNamespacedIdentityAcrossNamespaces(t *testing.T) {
	// A namespaced CEL policy name is unique within its namespace: two namespaces each holding
	// a policy of the same name are two distinct resources, not a duplicate.
	srcDir := writeTree(t, map[string]string{
		"team-a.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: NamespacedValidatingPolicy
metadata:
  name: check-pod
  namespace: team-a
spec:
  validations:
  - expression: "true"
`,
		"team-b.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: NamespacedValidatingPolicy
metadata:
  name: check-pod
  namespace: team-b
spec:
  validations:
  - expression: "true"
`,
	})
	dstDir := pushAndPull(t, srcDir)
	_, err := os.Stat(filepath.Join(dstDir, "team-a.yaml"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(dstDir, "team-b.yaml"))
	assert.NoError(t, err)
}

func TestReadRejectsDuplicateIdentity(t *testing.T) {
	srcDir := writeTree(t, map[string]string{
		"a.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: NamespacedValidatingPolicy
metadata:
  name: check-pod
  namespace: team-a
spec:
  validations:
  - expression: "true"
`,
		"b.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: NamespacedValidatingPolicy
metadata:
  name: check-pod
  namespace: team-a
spec:
  validations:
  - expression: "true"
`,
	})
	img := buildUnvalidatedImage(t, srcDir)
	_, err := bundle.Read(img, t.TempDir())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate resource identity")
}

func TestOptionsExecuteWithoutRegistry(t *testing.T) {
	o := options{imageRef: ""}
	err := o.validate("dir")
	assert.Error(t, err)

	o2 := options{imageRef: "example.com/repo/test:v1"}
	err = o2.validate("")
	assert.Error(t, err)
}

// TestOptionsExecuteFailsWhenImageDoesNotExist uses an in-process registry (no external network
// dependency, so it can't flake on DNS, TLS, or an intermediary) and asks for a reference that
// registry never received, so the failure is deterministically "manifest unknown", not whatever
// example.com happens to return today.
func TestOptionsExecuteFailsWhenImageDoesNotExist(t *testing.T) {
	srv := httptest.NewServer(registry.New())
	defer srv.Close()
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	dir := t.TempDir()
	o := options{imageRef: u.Host + "/does-not-exist/repo:v1"}
	err = o.execute(context.Background(), dir, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fetching remote image")
}
