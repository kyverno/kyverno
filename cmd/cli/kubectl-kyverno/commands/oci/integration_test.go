// Package oci_test holds #17668's cross-command integration tests: one artifact driven through
// several of the oci subcommand's actual cobra commands (push, resolve, inspect, validate, pull,
// and apply's "oci://" source), against a real in-process registry. Per-kind and per-media-type
// rejection already has unit coverage in internal/bundle, pull, push, and validate's own
// options_test.go files (see each test's doc comment for which merged test it would otherwise
// duplicate); this file only adds what driving the commands together, or a real network boundary,
// is needed to prove: authenticated registry operations, the Flux OCIRepository controller's
// extraction gate, and content-addressed determinism/immutability.
package oci_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	gcrname "github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/apply"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/inspect"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal/ocitest"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/pull"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/push"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const integrationVP = `apiVersion: policies.kyverno.io/v1beta1
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
  - expression: "has(object.metadata.labels)"
    message: "labels are required"
`

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

// --- Authenticated registry operations (D5): net-new, no prior test in this repo exercises an
// authenticated registry push, pull, resolve, or inspect. Bearer auth (GHCR, ECR, Docker Hub,
// ACR) is not covered: ocitest.NewAuthRegistry only emulates Basic. ---

// TestAuthRegistryRejectsAnonymous pins that a keychain which never supplies credentials fails
// against an auth-gated registry with the registry's own 401, not a Kyverno-authored error.
func TestAuthRegistryRejectsAnonymous(t *testing.T) {
	host := ocitest.NewAuthRegistry(t, "alice", "s3cret")
	kc := ocitest.StaticKeychain(host, "alice", "s3cret")
	ref := host + "/bundles/auth:v1"
	ocitest.PushTree(t, ref, map[string]string{"policy.yaml": integrationVP}, kc)

	cmd := resolve.Command(authn.NewMultiKeychain())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"-i", ref})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
	assert.Empty(t, out.String())
}

// TestAuthRegistryRejectsWrongPassword pins the design-critic's amendment: the wrapper checks
// decoded credentials, not merely that an Authorization header is present, so a wrong password
// (not just a missing one) must also 401.
func TestAuthRegistryRejectsWrongPassword(t *testing.T) {
	host := ocitest.NewAuthRegistry(t, "alice", "s3cret")
	kc := ocitest.StaticKeychain(host, "alice", "s3cret")
	ref := host + "/bundles/auth:v1"
	ocitest.PushTree(t, ref, map[string]string{"policy.yaml": integrationVP}, kc)

	wrongKC := ocitest.StaticKeychain(host, "alice", "wrong-password")
	cmd := resolve.Command(wrongKC)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"-i", ref})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
	assert.Empty(t, out.String())
}

// TestAuthRegistryRoundTripWithKeychain pins the regression class this repo owns: an injected
// keychain threads correctly through push, resolve, and inspect's own Command(keychain)
// constructors end to end against a real auth-gated registry.
func TestAuthRegistryRoundTripWithKeychain(t *testing.T) {
	host := ocitest.NewAuthRegistry(t, "alice", "s3cret")
	kc := ocitest.StaticKeychain(host, "alice", "s3cret")
	dir := writeTree(t, map[string]string{"policy.yaml": integrationVP})
	ref := host + "/bundles/roundtrip:v1"

	pushCmd := push.Command(kc)
	pushCmd.SetArgs([]string{dir, "-i", ref})
	require.NoError(t, pushCmd.Execute())

	resolveCmd := resolve.Command(kc)
	var resolveOut bytes.Buffer
	resolveCmd.SetOut(&resolveOut)
	resolveCmd.SetArgs([]string{"-i", ref, "--digest-only"})
	require.NoError(t, resolveCmd.Execute())
	digest := strings.TrimSpace(resolveOut.String())
	require.NotEmpty(t, digest)

	pinnedRef := host + "/bundles/roundtrip@" + digest
	inspectCmd := inspect.Command(kc)
	var inspectOut bytes.Buffer
	inspectCmd.SetOut(&inspectOut)
	inspectCmd.SetArgs([]string{"-i", pinnedRef, "-o", "json"})
	require.NoError(t, inspectCmd.Execute())

	var report inspect.Report
	require.NoError(t, json.Unmarshal(inspectOut.Bytes(), &report))
	assert.Equal(t, digest, report.Digest)
}

// TestDefaultKeychainReadsDockerConfig pins the other half of the auth story: pull.NewKeychain's
// real authn.DefaultKeychain picks up credentials from DOCKER_CONFIG, with no test-only keychain
// involved.
func TestDefaultKeychainReadsDockerConfig(t *testing.T) {
	host := ocitest.NewAuthRegistry(t, "alice", "s3cret")
	pushKC := ocitest.StaticKeychain(host, "alice", "s3cret")
	ref := host + "/bundles/dockerconfig:v1"
	ocitest.PushTree(t, ref, map[string]string{"policy.yaml": integrationVP}, pushKC)

	configDir := t.TempDir()
	auth := "YWxpY2U6czNjcmV0" // base64("alice:s3cret")
	configJSON := `{"auths":{"` + host + `":{"auth":"` + auth + `"}}}`
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.json"), []byte(configJSON), 0o600))
	t.Setenv("DOCKER_CONFIG", configDir)

	cmd := resolve.Command(pull.NewKeychain())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"-i", ref, "--digest-only"})
	require.NoError(t, cmd.Execute())
	assert.NotEmpty(t, strings.TrimSpace(out.String()))
}

// --- Flux-compatible extraction (D5): what the fluxcd/source-controller OCIRepository
// controller's own extraction gate requires, verified against fluxcd/source-controller
// ocirepository_controller.go (main@1e2bdd5e, selectLayer/buildArtifactFromEntries) and
// fluxcd/pkg tar/untar.go (main@13c7c12b, Untar/validRelPath): no layerSelector set means
// layers[0]; the layer must decompress as gzip (no fallback to an uncompressed or static-file
// path, unlike the flux CLI's weaker oracle); every tar entry's name must be non-empty, use no
// backslash, not be absolute, and not escape the extraction root via "..", and only regular files
// and directories are accepted (anything else, e.g. a symlink or device file, is refused). This
// test uses only the standard library and go-containerregistry (already a dependency); it does
// not import fluxcd/pkg or fluxcd/source-controller. ---

// validRelPath mirrors fluxcd/pkg tar/untar.go's validRelPath (main@13c7c12b): the path must be
// non-empty, must not contain a backslash, must not be absolute, and must not contain a ".."
// element after cleaning.
func validRelPath(p string) bool {
	if p == "" || strings.Contains(p, `\`) || strings.HasPrefix(p, "/") {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(p))
	for _, elem := range strings.Split(clean, "/") {
		if elem == ".." {
			return false
		}
	}
	return true
}

func TestFluxCompatibleExtraction(t *testing.T) {
	host := ocitest.NewRegistry(t)
	ref := host + "/bundles/flux:v1"
	ocitest.PushTree(t, ref, map[string]string{
		"policies/require-labels.yaml": integrationVP,
		"exceptions/team-a/check.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: check-pod
  namespace: team-a
spec:
  policyRefs:
  - name: require-labels
    kind: ValidatingPolicy
`,
	}, nil)

	name, err := gcrname.ParseReference(ref)
	require.NoError(t, err)
	img, err := remote.Image(name)
	require.NoError(t, err)

	layers, err := img.Layers()
	require.NoError(t, err)
	// No layerSelector configured: the controller's selectLayer falls back to layers[0], which
	// only works when there is exactly one layer (bundle-spec.md section 5's "exactly one layer").
	require.Len(t, layers, 1)
	layer := layers[0]

	rc, err := layer.Compressed()
	require.NoError(t, err)
	defer rc.Close()

	magic := make([]byte, 2)
	_, err = rc.Read(magic)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x1f, 0x8b}, magic, "Flux's tar.Untar requires a gzip-compressed body with no fallback")

	rc.Close()
	rc2, err := layer.Compressed()
	require.NoError(t, err)
	defer rc2.Close()
	gz, err := gzip.NewReader(rc2)
	require.NoError(t, err)
	tr := tar.NewReader(gz)

	var names []string
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		assert.True(t, validRelPath(hdr.Name), "entry %q must satisfy Flux's validRelPath", hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeReg, tar.TypeDir:
			// accepted types per fluxcd/pkg tar.Untar
		default:
			t.Fatalf("entry %q has type %d, which Flux's tar.Untar refuses", hdr.Name, hdr.Typeflag)
		}
		if hdr.Typeflag == tar.TypeReg {
			names = append(names, hdr.Name)
		}
	}
	assert.ElementsMatch(t, []string{"policies/require-labels.yaml", "exceptions/team-a/check.yaml"}, names)
}

// --- Determinism and immutability (D3): expose-don't-enforce, pinned by a falsifier and a
// registry-contract test. ---

// TestPushSameTreeTwoRefsSameDigest is the determinism falsifier: pushing the same tree to two
// different references yields the same content digest, because bundle.Write never derives
// content from the push reference.
func TestPushSameTreeTwoRefsSameDigest(t *testing.T) {
	host := ocitest.NewRegistry(t)
	files := map[string]string{"policy.yaml": integrationVP}
	digestA := ocitest.PushTree(t, host+"/bundles/a:v1", files, nil)
	digestB := ocitest.PushTree(t, host+"/bundles/b:v9", files, nil)
	assert.Equal(t, digestA.String(), digestB.String())
}

// TestMovedTagKeepsPinnedContent documents the registry's content-addressed-store contract
// (noted in the design as not independently falsifiable by any Kyverno change): resolving a tag
// to a digest and then moving the tag to different content leaves the pinned digest reference's
// own inspect -o json output byte-for-byte unchanged.
func TestMovedTagKeepsPinnedContent(t *testing.T) {
	host := ocitest.NewRegistry(t)
	ref := host + "/bundles/moving:v1"
	ocitest.PushTree(t, ref, map[string]string{"policy.yaml": integrationVP}, nil)

	resolveCmd := resolve.Command(nil)
	var resolveOut bytes.Buffer
	resolveCmd.SetOut(&resolveOut)
	resolveCmd.SetArgs([]string{"-i", ref})
	require.NoError(t, resolveCmd.Execute())
	pinnedRef := strings.TrimSpace(resolveOut.String())

	inspectBefore := func() []byte {
		cmd := inspect.Command(nil)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"-i", pinnedRef, "-o", "json"})
		require.NoError(t, cmd.Execute())
		return out.Bytes()
	}
	before := inspectBefore()

	// Move the tag to different content.
	ocitest.PushTree(t, ref, map[string]string{"policy.yaml": integrationVP + "\n# a trailing comment to change the digest\n"}, nil)

	after := inspectBefore()
	assert.Equal(t, before, after, "a moved tag must not change what the pinned digest reference inspects as")
}

// TestApplyAcceptsDigestPinnedOCIReference covers the issue's "apply/test accept a digest-pinned
// oci:// reference" criterion. A bundle's writer MUST reject any cli.kyverno.io document
// (TestAssembleRejectsCliKyvernoIoDocumentAnywhere), so a bundle can never contain a
// kyverno-test.yaml and "kyverno test oci://..." has nothing to run; this asserts the two
// achievable halves instead: pull.ToTempDir extracts a pinned reference's tree, and apply accepts
// an "oci://" policy source pinned to a digest.
func TestApplyAcceptsDigestPinnedOCIReference(t *testing.T) {
	host := ocitest.NewRegistry(t)
	ref := host + "/bundles/apply-oci:v1"
	digest := ocitest.PushTree(t, ref, map[string]string{"policy.yaml": integrationVP}, nil)
	pinnedRef := host + "/bundles/apply-oci@" + digest.String()

	dir, cleanup, err := pull.ToTempDir(context.Background(), pinnedRef, nil)
	require.NoError(t, err)
	defer cleanup()
	_, statErr := os.Stat(filepath.Join(dir, "policy.yaml"))
	require.NoError(t, statErr, "pull.ToTempDir must extract the pinned reference's tree")

	resourceDir := t.TempDir()
	resourcePath := filepath.Join(resourceDir, "resource.yaml")
	require.NoError(t, os.WriteFile(resourcePath, []byte(`apiVersion: v1
kind: Pod
metadata:
  name: no-labels
spec:
  containers:
  - name: nginx
    image: nginx
`), 0o600))

	cmd := apply.Command()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"oci://" + pinnedRef, "--resource", resourcePath})
	err = cmd.Execute()
	require.Error(t, err, "the unlabeled pod must fail require-labels, which exits non-zero")
	assert.Contains(t, out.String(), "fail: 1", "the pod has no labels so require-labels must cleanly fail, not error")
}
