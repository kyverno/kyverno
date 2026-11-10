package apply

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const errorOutputTestValidatingPolicy = `
apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: require-team-annotation
spec:
  validationActions: [Deny]
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [CREATE]
      resources: [pods]
  validations:
  - expression: object.metadata.annotations["team"] != ""
    message: a team annotation is required
`

const errorOutputTestPod = `
apiVersion: v1
kind: Pod
metadata:
  name: pod
  namespace: default
spec:
  containers:
  - name: c
    image: nginx:latest
`

// kyverno apply's default (non-table) text output must print the rule's message when a
// Validation-type rule errors, same as it already does for Mutation and Generation rules -
// reading a missing annotations key is a CEL evaluation error (RuleStatusError), not a failed
// validation (RuleStatusFail), since the policy never ran the comparison at all.
func TestCommandValidatingPolicyErrorIsPrinted(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	resourcePath := filepath.Join(dir, "resource.yaml")
	require.NoError(t, os.WriteFile(policyPath, []byte(errorOutputTestValidatingPolicy), 0o600))
	require.NoError(t, os.WriteFile(resourcePath, []byte(errorOutputTestPod), 0o600))

	cmd := Command()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{policyPath, "--resource", resourcePath})
	err := cmd.Execute()
	require.Error(t, err)

	output := out.String()
	assert.Contains(t, output, "pass: 0, fail: 0, warn: 0, error: 1, skip: 0")
	assert.Contains(t, output, "error while applying policy")
	assert.Contains(t, output, "require-team-annotation")
	assert.Contains(t, output, "no such key")
}
