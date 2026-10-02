package processor

import (
	"bytes"
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
  mutations:
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >
        Object{metadata: Object.metadata{labels: {"team": "a"}}}
`

// A Pod policy with autogen also produces an empty response for its pod controller copy.
// One mutation must be counted, printed and written once.
func Test_ApplyPoliciesOnResource_mutatingPolicyCountedOnce(t *testing.T) {
	var mpol policiesv1beta1.MutatingPolicy
	assert.NilError(t, yaml.Unmarshal([]byte(podLabelMutatingPolicy), &mpol))
	resources, err := resource.GetUnstructuredResources([]byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"},"spec":{"containers":[{"name":"c","image":"nginx:latest"}]}}`))
	assert.NilError(t, err)

	// The apply command creates the output file before resources are processed.
	outputFile := filepath.Join(t.TempDir(), "mutated.yaml")
	assert.NilError(t, os.WriteFile(outputFile, nil, 0o600))
	out := &bytes.Buffer{}
	rc := &ResultCounts{}
	p := PolicyProcessor{
		Store:              &store.Store{},
		MutatingPolicies:   []policiesv1beta1.MutatingPolicyLike{&mpol},
		Resource:           *resources[0],
		Variables:          &variables.Variables{},
		Rc:                 rc,
		Out:                out,
		MutateLogPath:      outputFile,
		MutateLogPathIsDir: false,
	}
	_, err = p.ApplyPoliciesOnResource()
	assert.NilError(t, err)

	assert.Equal(t, rc.Pass, 1)
	assert.Equal(t, strings.Count(out.String(), "Mutation has been applied successfully"), 1)
	written, err := os.ReadFile(outputFile)
	assert.NilError(t, err)
	assert.Equal(t, strings.Count(string(written), "kind: Pod"), 1)
}
