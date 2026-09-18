package test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A test file that declares only `checks:` (no `results:`) must still be
// executed. The check below asserts `status: pass` while the Pod genuinely
// fails the policy, so the run must report a failure.
func TestChecksOnlyTestFileIsExecuted(t *testing.T) {
	color.Init(true)
	dir := t.TempDir()
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}
	write("policy.yaml", `
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-labels
spec:
  validationFailureAction: Enforce
  rules:
  - name: check-team
    match:
      any:
      - resources:
          kinds: [Pod]
    validate:
      message: "label 'team' is required"
      pattern:
        metadata:
          labels:
            team: "?*"
`)
	write("resource.yaml", `
apiVersion: v1
kind: Pod
metadata:
  name: badpod
spec:
  containers:
  - name: c
    image: nginx
`)
	write("kyverno-test.yaml", `
apiVersion: cli.kyverno.io/v1alpha1
kind: Test
metadata:
  name: checks-only
policies: [policy.yaml]
resources: [resource.yaml]
checks:
- match:
    resource:
      kind: Pod
  assert:
    status: pass
`)

	err := testCommandExecute(io.Discard, []string{dir}, "kyverno-test.yaml", "", "policy=*,rule=*,resource=*", "", false, false, false, false, true)
	assert.Error(t, err, "checks-only test file was silently skipped: the failing assertion was never evaluated")
}
