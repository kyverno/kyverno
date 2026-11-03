package validate

import (
	"context"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	gcrname "github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal/bundle"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/pull"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTree writes files (bundle-root-relative path -> content) under a fresh temp directory and
// returns its path, matching push/options_test.go's helper.
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
  - expression: "object.metadata.labels != null"
    message: "labels are required"
`

// TestValidateLocalDirSucceedsWithoutImage pins D1's local mode: no --image is required, and a
// well-formed tree validates successfully with no network call.
func TestValidateLocalDirSucceedsWithoutImage(t *testing.T) {
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	o := options{}
	err := o.execute(context.Background(), []string{dir}, nil)
	assert.NoError(t, err)
}

// TestValidateLocalDirReportsPushEquivalentError pins D1's headline contract: local validate
// reports exactly what push would reject for the same tree, using bundle.Validate's own error
// message as the oracle, over a bundle whose exception references a policy outside the bundle
// (push's own strict default, writer MUST 12).
func TestValidateLocalDirReportsPushEquivalentError(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"exception.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: exempt-labels
spec:
  policyRefs:
  - name: some-policy-not-in-this-bundle
    kind: ValidatingPolicy
`,
	})

	b, err := bundle.Assemble(dir)
	require.NoError(t, err)
	wantErr := bundle.Validate(b)
	require.Error(t, wantErr)

	o := options{}
	gotErr := o.execute(context.Background(), []string{dir}, nil)
	require.Error(t, gotErr)
	assert.Equal(t, wantErr.Error(), gotErr.Error(), "validate must report exactly the error push's own Validate call would")
}

// TestValidateLocalDirFailsOnUnparseableSourceDateEpoch pins D1's reason for running Write in
// local mode: resolveCreated's SOURCE_DATE_EPOCH parse failure is the only remaining pre-upload
// step of push that can fail on input, so without running Write the push-equivalence contract
// above would be false for this input.
func TestValidateLocalDirFailsOnUnparseableSourceDateEpoch(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "not-a-number")
	dir := writeTree(t, map[string]string{"policy.yaml": validVP})
	o := options{}
	err := o.execute(context.Background(), []string{dir}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "writing bundle image")
}

// TestValidateLocalDirRejectsLegacyKind pins that local mode surfaces the same migration-hint
// rejection Assemble's loader already gives push for a legacy kyverno.io kind.
func TestValidateLocalDirRejectsLegacyKind(t *testing.T) {
	dir := writeTree(t, map[string]string{
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
	o := options{}
	err := o.execute(context.Background(), []string{dir}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v1")
}

// TestValidateLocalDirAcceptsV1Policy and TestValidateLocalDirRejectsV1alpha1Policy pin D4
// together: the widened accepted-version table takes v1, and v1alpha1 stays rejected with an
// actionable error naming the accepted set.
func TestValidateLocalDirAcceptsV1Policy(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"policy.yaml": `apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: require-labels
spec:
  validations:
  - expression: "true"
`,
	})
	o := options{}
	err := o.execute(context.Background(), []string{dir}, nil)
	assert.NoError(t, err)
}

func TestValidateLocalDirRejectsV1alpha1Policy(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"policy.yaml": `apiVersion: policies.kyverno.io/v1alpha1
kind: ValidatingPolicy
metadata:
  name: require-labels
spec:
  validations:
  - expression: "true"
`,
	})
	o := options{}
	err := o.execute(context.Background(), []string{dir}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "policies.kyverno.io/v1alpha1")
	assert.Contains(t, err.Error(), internal.AcceptedAPIVersionsText())
}

// TestValidateLocalDirRejectsMalformedSelector pins D3: a structurally malformed
// matchConstraints.namespaceSelector on a policy fails validate the same way it fails push.
func TestValidateLocalDirRejectsMalformedSelector(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"policy.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: bad-selector
spec:
  matchConstraints:
    namespaceSelector:
      matchExpressions:
      - key: env
        operator: NotAnOperator
        values: ["prod"]
  validations:
  - expression: "true"
`,
	})
	o := options{}
	err := o.execute(context.Background(), []string{dir}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "namespaceSelector")
}

// TestValidateDefaultRunOnlyWarnsOnExpiredException and TestValidateStrictFailsOnWarnings pin D2:
// expiry is a warning by default (validate succeeds, exit code 0) and a failure only under
// --strict, which promotes every warning the run emits to a hard failure.
func expiredExceptionTree(t *testing.T) string {
	t.Helper()
	return writeTree(t, map[string]string{
		"policy.yaml": validVP,
		"exception.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: expired-exception
spec:
  expiresAt: "2000-01-01T00:00:00Z"
  policyRefs:
  - name: require-labels
    kind: ValidatingPolicy
`,
	})
}

func TestValidateDefaultRunOnlyWarnsOnExpiredException(t *testing.T) {
	dir := expiredExceptionTree(t)
	o := options{}
	err := o.execute(context.Background(), []string{dir}, nil)
	assert.NoError(t, err, "an expired exception is a warning, not a failure, without --strict")
}

func TestValidateStrictFailsOnWarnings(t *testing.T) {
	dir := expiredExceptionTree(t)
	o := options{strict: true}
	err := o.execute(context.Background(), []string{dir}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--strict is set")
}

// TestValidateKyvernoVersionFlagRejectsNonRelease pins D5's options.validate guard: a
// --kyverno-version value that isn't a release version is a flag error, caught before execute
// ever runs.
func TestValidateKyvernoVersionFlagRejectsNonRelease(t *testing.T) {
	o := options{kyvernoTarget: "---"}
	err := o.validate([]string{"some-dir"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--kyverno-version")
	assert.Contains(t, err.Error(), "not a release version")
}

// TestValidateKyvernoVersionFlagFailsOnUnsatisfied pins D5: an explicit --kyverno-version target
// turns an Unsatisfied comparison into a hard failure, unlike the default run (push's own
// comparison against the CLI's own version), which only warns.
func TestValidateKyvernoVersionFlagFailsOnUnsatisfied(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"policy.yaml": validVP,
		internal.DescriptorFilename: `apiVersion: cli.kyverno.io/v1alpha1
kind: PolicyBundle
metadata:
  name: baseline
spec:
  kyvernoVersion: ">=99.0.0"
`,
	})
	o := options{kyvernoTarget: "1.20.0"}
	err := o.execute(context.Background(), []string{dir}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not satisfy")
	assert.Contains(t, err.Error(), "1.20.0")
}

// TestValidateMutualExclusionPathAndImage and TestValidateRequiresPathOrImage pin D1's options.validate
// gate directly (command_test.go exercises the same rule end to end through cobra).
func TestValidateMutualExclusionPathAndImage(t *testing.T) {
	o := options{imageRef: "example.com/repo/test:v1"}
	err := o.validate([]string{"some-dir"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not both")
}

func TestValidateRequiresPathOrImage(t *testing.T) {
	o := options{}
	err := o.validate(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is required")
}

// TestValidateReferenceModeRunsStrictChecks pins D1's reference mode contract: it applies the same
// strict checks local mode does over a pushed image, so it fails on content "kyverno oci pull"
// accepts -- a PolicyException referencing a policy outside the bundle (format-legal; push's
// strict default is what rejects it, not the format). The image is pushed to an in-process
// registry via bundle.Write directly (bypassing bundle.Validate, as a non-conformant or
// already-published writer might have), so the only thing standing between "pull succeeds" and
// "validate -i fails" is validate's own re-application of ValidateWithOptions.
func TestValidateReferenceModeRunsStrictChecks(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"exception.yaml": `apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: exempt-labels
spec:
  policyRefs:
  - name: some-policy-not-in-this-bundle
    kind: ValidatingPolicy
`,
	})
	b, err := bundle.Assemble(dir)
	require.NoError(t, err)
	img, err := bundle.Write(b, nil)
	require.NoError(t, err)

	srv := httptest.NewServer(registry.New())
	defer srv.Close()
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	ref := u.Host + "/bundles/dangling:v1"

	name, err := gcrname.ParseReference(ref)
	require.NoError(t, err)
	require.NoError(t, remote.Write(name, img))

	// pull succeeds: the format itself permits a dangling in-bundle reference (writer MUST 12's
	// carve-out is push's own strict default, not a reader requirement).
	pulledImg, _, fetchErr := pull.FetchImage(context.Background(), ref, nil)
	require.NoError(t, fetchErr)
	pullDir := t.TempDir()
	_, readErr := bundle.Read(pulledImg, pullDir)
	assert.NoError(t, readErr, "pull must accept a format-legal dangling reference")

	// validate -i re-applies push's strict checks and must fail on the same content.
	o := options{imageRef: ref}
	err = o.execute(context.Background(), nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "references unknown policy")
}
