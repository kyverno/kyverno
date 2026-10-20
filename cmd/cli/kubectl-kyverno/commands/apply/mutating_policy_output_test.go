package apply

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
