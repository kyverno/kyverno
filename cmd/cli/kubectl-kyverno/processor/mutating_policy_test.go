package processor

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Test_ApplyPoliciesOnResource_targetConstrainedMutatingPolicySkipsTrigger verifies that a
// MutatingPolicy with active targetMatchConstraints is not evaluated inline against its
// trigger, as admission does, so only the declared target receives the mutation.
func Test_ApplyPoliciesOnResource_targetConstrainedMutatingPolicySkipsTrigger(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "..", "test", "cli", "test-mutating-policy", "mutate-existing")
	results, err := policy.Load(nil, "", true, filepath.Join(dir, "policy.yaml"))
	assert.NilError(t, err)
	assert.Equal(t, len(results.MutatingPolicies), 1)

	load := func(name string) *unstructured.Unstructured {
		data, err := os.ReadFile(filepath.Join(dir, name))
		assert.NilError(t, err)
		resources, err := resource.GetUnstructuredResources(data)
		assert.NilError(t, err)
		assert.Equal(t, len(resources), 1)
		return resources[0]
	}
	trigger := load("trigger.yaml")
	target := load("target.yaml")

	processor := PolicyProcessor{
		Store:            &store.Store{},
		MutatingPolicies: results.MutatingPolicies,
		Resource:         *trigger,
		TargetResources:  []*unstructured.Unstructured{target},
		Variables:        &variables.Variables{},
		Rc:               &ResultCounts{},
		Out:              io.Discard,
	}

	responses, err := processor.ApplyPoliciesOnResource()
	assert.NilError(t, err)
	// Only the mutate-existing evaluation of the target remains; an inline evaluation
	// would add a response for the trigger carrying the target-shaped patch.
	assert.Equal(t, len(responses), 1, "unexpected inline evaluation of trigger %s", trigger.GetName())
	assert.Equal(t, responses[0].Resource.GetKind(), target.GetKind())
	assert.Equal(t, responses[0].Resource.GetName(), target.GetName())
}
