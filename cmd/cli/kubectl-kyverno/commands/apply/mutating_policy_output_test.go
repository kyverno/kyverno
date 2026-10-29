package apply

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fatihcolor "github.com/fatih/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const outputTestMutatingPolicy = `
apiVersion: policies.kyverno.io/v1beta1
kind: MutatingPolicy
metadata:
  name: add-team-label
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [CREATE]
      resources: [pods]
  mutations:
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >
        Object{metadata: Object.metadata{labels: {"team": %s}}}
`

const outputTestPod = `
apiVersion: v1
kind: Pod
metadata:
  name: pod
  namespace: default
  annotations: {}
spec:
  containers:
  - name: c
    image: nginx:latest
`

// kyverno apply -o must report, print and write one result for one MutatingPolicy applied to one Pod.
func TestCommandWithMutatingPolicyOutputFile(t *testing.T) {
	tests := []struct {
		name        string
		label       string
		wantSummary string
		wantErr     string
	}{{
		name:        "pass",
		label:       `"a"`,
		wantSummary: "pass: 1, fail: 0, warn: 0, error: 0, skip: 0",
	}, {
		name:        "error",
		label:       `object.metadata.annotations["missing"]`,
		wantSummary: "pass: 0, fail: 0, warn: 0, error: 1, skip: 0",
		wantErr:     "exit as there are policy errors",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			policyPath := filepath.Join(dir, "policy.yaml")
			resourcePath := filepath.Join(dir, "resource.yaml")
			outputPath := filepath.Join(dir, "mutated.yaml")
			require.NoError(t, os.WriteFile(policyPath, []byte(fmt.Sprintf(outputTestMutatingPolicy, tt.label)), 0o600))
			require.NoError(t, os.WriteFile(resourcePath, []byte(outputTestPod), 0o600))

			cmd := Command()
			out := &bytes.Buffer{}
			cmd.SetOut(out)
			cmd.SetErr(out)
			cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "-o", outputPath})
			err := cmd.Execute()
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Contains(t, out.String(), tt.wantSummary)
			assert.Equal(t, 1, strings.Count(out.String(), "Mutation has been applied successfully"))
			written, err := os.ReadFile(outputPath)
			require.NoError(t, err)
			assert.Equal(t, 1, strings.Count(string(written), "kind: Pod"))
		})
	}
}

const outputTestMutatingPolicyWithSkip = `
apiVersion: policies.kyverno.io/v1beta1
kind: MutatingPolicy
metadata:
  name: skip-mutate-policy
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [CREATE]
      resources: [pods]
  matchConditions:
  - name: is-dev-namespace
    expression: request.namespace == 'dev'
  mutations:
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >
        Object{metadata: Object.metadata{labels: {"team": "a"}}}
`

func TestMutatingPolicyColorAndRemoveColor(t *testing.T) {
	origNoColor := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = origNoColor
		color.Init(origNoColor)
	})

	dir := t.TempDir()
	resourcePath := filepath.Join(dir, "resource.yaml")
	require.NoError(t, os.WriteFile(resourcePath, []byte(outputTestPod), 0o600))

	errorPolicyPath := filepath.Join(dir, "error-policy.yaml")
	require.NoError(t, os.WriteFile(errorPolicyPath, []byte(fmt.Sprintf(outputTestMutatingPolicy, `object.metadata.annotations["missing"]`)), 0o600))

	skipPolicyPath := filepath.Join(dir, "skip-policy.yaml")
	require.NoError(t, os.WriteFile(skipPolicyPath, []byte(outputTestMutatingPolicyWithSkip), 0o600))

	t.Run("error with color", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		fatihcolor.NoColor = false
		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{errorPolicyPath, "--resource", resourcePath})
		err := cmd.Execute()
		require.Error(t, err)

		output := out.String()
		assert.Contains(t, output, "\x1b[31;1merror while applying mutate policy\x1b[0;22m")
		assert.Contains(t, output, "\x1b[36;1madd-team-label\x1b[0;22m")
		assert.Contains(t, output, "\x1b[31;1merror:\x1b[0;22m")
	})

	t.Run("error with remove-color", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		fatihcolor.NoColor = false
		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{errorPolicyPath, "--resource", resourcePath, "--remove-color"})
		err := cmd.Execute()
		require.Error(t, err)

		output := out.String()
		assert.Contains(t, output, "error while applying mutate policy")
		assert.Contains(t, output, "add-team-label")
		assert.Contains(t, output, "error:")
		assert.NotContains(t, output, "\x1b[")
	})

	t.Run("skip with color", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		fatihcolor.NoColor = false
		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{skipPolicyPath, "--resource", resourcePath})
		err := cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "\x1b[33;1mskipped mutate policy\x1b[0;22m")
		assert.Contains(t, output, "\x1b[36;1mskip-mutate-policy\x1b[0;22m")
	})

	t.Run("skip with remove-color", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		fatihcolor.NoColor = false
		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{skipPolicyPath, "--resource", resourcePath, "--remove-color"})
		err := cmd.Execute()
		require.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "skipped mutate policy")
		assert.Contains(t, output, "skip-mutate-policy")
		assert.NotContains(t, output, "\x1b[")
	})
}
