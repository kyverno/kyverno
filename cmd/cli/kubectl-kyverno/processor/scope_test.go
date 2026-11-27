package processor

import (
	"io"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	"gotest.tools/v3/assert"
	"sigs.k8s.io/yaml"
)

const namespacedValidatingPolicy = `
apiVersion: policies.kyverno.io/v1beta1
kind: NamespacedValidatingPolicy
metadata:
  name: require-team-label
  namespace: team-a
spec:
  validationActions: [Deny]
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [CREATE, UPDATE]
      resources: [pods]
  validations:
  - expression: "has(object.metadata.labels) && 'team' in object.metadata.labels"
    message: team label required
`

const namespacedMutatingPolicy = `
apiVersion: policies.kyverno.io/v1beta1
kind: NamespacedMutatingPolicy
metadata:
  name: add-team-label
  namespace: team-a
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

func podInNamespace(t *testing.T, namespace string) []byte {
	t.Helper()
	return []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"` + namespace + `"},"spec":{"containers":[{"name":"c","image":"nginx:latest"}]}}`)
}

func Test_InNamespaceScope(t *testing.T) {
	clustered := &policiesv1beta1.ValidatingPolicy{}
	namespaced := &policiesv1beta1.NamespacedValidatingPolicy{}
	namespaced.Namespace = "team-a"

	inTeamA := InNamespaceScope[policiesv1beta1.ValidatingPolicyLike]("team-a")
	inTeamB := InNamespaceScope[policiesv1beta1.ValidatingPolicyLike]("team-b")
	clusterScoped := InNamespaceScope[policiesv1beta1.ValidatingPolicyLike]("")

	assert.Assert(t, inTeamA(clustered))
	assert.Assert(t, inTeamB(clustered))
	assert.Assert(t, clusterScoped(clustered))
	assert.Assert(t, inTeamA(namespaced))
	assert.Assert(t, !inTeamB(namespaced))
	assert.Assert(t, !clusterScoped(namespaced))
}

// A namespaced policy is only evaluated against resources in its own namespace,
// the way the admission webhooks scope it in a cluster.
func Test_ApplyPoliciesOnResource_namespacedPolicyScope(t *testing.T) {
	var vpol policiesv1beta1.NamespacedValidatingPolicy
	assert.NilError(t, yaml.Unmarshal([]byte(namespacedValidatingPolicy), &vpol))
	var mpol policiesv1beta1.NamespacedMutatingPolicy
	assert.NilError(t, yaml.Unmarshal([]byte(namespacedMutatingPolicy), &mpol))

	tests := []struct {
		name      string
		namespace string
		processor func(PolicyProcessor) PolicyProcessor
		responses int
	}{{
		name:      "validating policy, resource in the policy namespace",
		namespace: "team-a",
		processor: func(p PolicyProcessor) PolicyProcessor {
			p.ValidatingPolicies = []policiesv1beta1.ValidatingPolicyLike{&vpol}
			return p
		},
		responses: 1,
	}, {
		name:      "validating policy, resource in another namespace",
		namespace: "team-b",
		processor: func(p PolicyProcessor) PolicyProcessor {
			p.ValidatingPolicies = []policiesv1beta1.ValidatingPolicyLike{&vpol}
			return p
		},
		responses: 0,
	}, {
		name:      "mutating policy, resource in the policy namespace",
		namespace: "team-a",
		processor: func(p PolicyProcessor) PolicyProcessor {
			p.MutatingPolicies = []policiesv1beta1.MutatingPolicyLike{&mpol}
			return p
		},
		responses: 1,
	}, {
		name:      "mutating policy, resource in another namespace",
		namespace: "team-b",
		processor: func(p PolicyProcessor) PolicyProcessor {
			p.MutatingPolicies = []policiesv1beta1.MutatingPolicyLike{&mpol}
			return p
		},
		responses: 0,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources, err := resource.GetUnstructuredResources(podInNamespace(t, tt.namespace))
			assert.NilError(t, err)
			p := tt.processor(PolicyProcessor{
				Store:     &store.Store{},
				Resource:  *resources[0],
				Variables: &variables.Variables{},
				Rc:        &ResultCounts{},
				Out:       io.Discard,
			})
			responses, err := p.ApplyPoliciesOnResource()
			assert.NilError(t, err)
			matched := 0
			for _, response := range responses {
				if len(response.PolicyResponse.Rules) > 0 {
					matched++
				}
			}
			assert.Equal(t, matched, tt.responses)
		})
	}
}
