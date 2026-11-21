package test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChecksOnly(t *testing.T) {
	policyYAML := `apiVersion: policies.kyverno.io/v1alpha1
kind: ValidatingPolicy
metadata:
  name: require-team-label
spec:
  matchConstraints:
    resourceRules:
      - apiGroups: [""]
        apiVersions: ["v1"]
        operations: ["CREATE", "UPDATE"]
        resources: ["pods"]
  validations:
    - expression: "object.metadata.?labels['team'].orValue('') != ''"
      message: "The label 'team' is required."
`

	goodPodYAML := `apiVersion: v1
kind: Pod
metadata:
  name: goodpod
  namespace: default
  labels:
    team: dev
spec:
  containers:
  - name: c
    image: nginx
`

	badPodYAML := `apiVersion: v1
kind: Pod
metadata:
  name: badpod
  namespace: default
spec:
  containers:
  - name: c
    image: nginx
`

	t.Run("checks-only passing test is executed and succeeds", func(t *testing.T) {
		tempDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "policy.yaml"), []byte(policyYAML), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "resource.yaml"), []byte(goodPodYAML), 0600))

		testYAML := `apiVersion: cli.kyverno.io/v1alpha1
kind: Test
metadata:
  name: checks-only-pass
policies:
  - policy.yaml
resources:
  - resource.yaml
checks:
  - match:
      resource:
        metadata:
          name: goodpod
    assert:
      status: pass
`
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "kyverno-test.yaml"), []byte(testYAML), 0600))

		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{tempDir, "--remove-color"})

		err := cmd.Execute()
		require.NoError(t, err)
		output := out.String()
		assert.Contains(t, output, "1 tests passed and 0 tests failed")
	})

	t.Run("checks-only failing assertion causes test failure", func(t *testing.T) {
		tempDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "policy.yaml"), []byte(policyYAML), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "resource.yaml"), []byte(badPodYAML), 0600))

		// The policy will fail on badpod (missing team label), but check asserts status == pass.
		// This should cause the check assertion to fail and the CLI to exit with an error.
		testYAML := `apiVersion: cli.kyverno.io/v1alpha1
kind: Test
metadata:
  name: checks-only-fail
policies:
  - policy.yaml
resources:
  - resource.yaml
checks:
  - match:
      resource:
        metadata:
          name: badpod
    assert:
      status: pass
`
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "kyverno-test.yaml"), []byte(testYAML), 0600))

		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{tempDir, "--remove-color"})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "1 tests failed")
		output := out.String()
		assert.Contains(t, output, "0 tests passed and 1 tests failed")
	})

	t.Run("checks-only with require-tests does not error as missing tests", func(t *testing.T) {
		tempDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "policy.yaml"), []byte(policyYAML), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "resource.yaml"), []byte(goodPodYAML), 0600))

		testYAML := `apiVersion: cli.kyverno.io/v1alpha1
kind: Test
metadata:
  name: checks-only-require
policies:
  - policy.yaml
resources:
  - resource.yaml
checks:
  - match:
      resource:
        metadata:
          name: goodpod
    assert:
      status: pass
`
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "kyverno-test.yaml"), []byte(testYAML), 0600))

		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{tempDir, "--remove-color", "--require-tests"})

		err := cmd.Execute()
		require.NoError(t, err)
		assert.Contains(t, out.String(), "1 tests passed and 0 tests failed")
	})

	t.Run("check row numbering continues after result rows", func(t *testing.T) {
		tempDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "policy.yaml"), []byte(policyYAML), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "resource.yaml"), []byte(goodPodYAML), 0600))

		testYAML := `apiVersion: cli.kyverno.io/v1alpha1
kind: Test
metadata:
  name: mixed-results-checks
policies:
  - policy.yaml
resources:
  - resource.yaml
results:
  - isValidatingPolicy: true
    policy: require-team-label
    resources:
      - goodpod
    kind: Pod
    result: pass
checks:
  - match:
      resource:
        metadata:
          name: goodpod
    assert:
      status: pass
`
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "kyverno-test.yaml"), []byte(testYAML), 0600))

		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{tempDir, "--remove-color"})

		err := cmd.Execute()
		require.NoError(t, err)
		output := out.String()
		assert.Contains(t, output, "2 tests passed and 0 tests failed")
	})
}
