package processor

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	"gotest.tools/v3/assert"
	"sigs.k8s.io/yaml"
)

const podLabelMutatingPolicy = `
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
  matchConditions:
  - name: condition
    expression: %s
  mutations:
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >
        Object{metadata: Object.metadata{labels: {"team": %s}}}
`

func newPodLabelMutatingPolicy(t *testing.T, condition, label string) *policiesv1beta1.MutatingPolicy {
	t.Helper()
	var mpol policiesv1beta1.MutatingPolicy
	assert.NilError(t, yaml.Unmarshal([]byte(fmt.Sprintf(podLabelMutatingPolicy, condition, label)), &mpol))
	return &mpol
}

// A Pod policy with autogen also produces an empty response for its pod controller copy.
// Each result must be counted, printed and written once, whatever its status.
func Test_ApplyPoliciesOnResource_mutatingPolicyCountedOnce(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		label     string
		want      ResultCounts
	}{{
		name:      "pass",
		condition: "'true'",
		label:     `"a"`,
		want:      ResultCounts{Pass: 1},
	}, {
		name:      "skip",
		condition: "'false'",
		label:     `"a"`,
		want:      ResultCounts{Skip: 1},
	}, {
		name:      "error",
		condition: "'true'",
		label:     `object.metadata.annotations["missing"]`,
		want:      ResultCounts{Error: 1},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources, err := resource.GetUnstructuredResources([]byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default","annotations":{}},"spec":{"containers":[{"name":"c","image":"nginx:latest"}]}}`))
			assert.NilError(t, err)

			// The apply command creates the output file before resources are processed.
			outputFile := filepath.Join(t.TempDir(), "mutated.yaml")
			assert.NilError(t, os.WriteFile(outputFile, nil, 0o600))
			out := &bytes.Buffer{}
			rc := &ResultCounts{}
			p := PolicyProcessor{
				Store:              &store.Store{},
				MutatingPolicies:   []policiesv1beta1.MutatingPolicyLike{newPodLabelMutatingPolicy(t, tt.condition, tt.label)},
				Resource:           *resources[0],
				Variables:          &variables.Variables{},
				Rc:                 rc,
				Out:                out,
				MutateLogPath:      outputFile,
				MutateLogPathIsDir: false,
			}
			_, err = p.ApplyPoliciesOnResource()
			assert.NilError(t, err)

			assert.DeepEqual(t, *rc, tt.want)
			assert.Equal(t, strings.Count(out.String(), "Mutation has been applied successfully"), 1)
			written, err := os.ReadFile(outputFile)
			assert.NilError(t, err)
			assert.Equal(t, strings.Count(string(written), "kind: Pod"), 1)
		})
	}
}

func Test_ApplyPoliciesOnResource_mutatingPolicyOutputError(t *testing.T) {
	resources, err := resource.GetUnstructuredResources([]byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"},"spec":{"containers":[{"name":"c","image":"nginx:latest"}]}}`))
	assert.NilError(t, err)
	p := PolicyProcessor{
		Store:              &store.Store{},
		MutatingPolicies:   []policiesv1beta1.MutatingPolicyLike{newPodLabelMutatingPolicy(t, "'true'", `"a"`)},
		Resource:           *resources[0],
		Variables:          &variables.Variables{},
		Rc:                 &ResultCounts{},
		Out:                &bytes.Buffer{},
		MutateLogPath:      filepath.Join(t.TempDir(), "missing", "mutated.yaml"),
		MutateLogPathIsDir: false,
	}
	_, err = p.ApplyPoliciesOnResource()
	assert.ErrorContains(t, err, "failed to print mutated result")
}
