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
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"gotest.tools/v3/assert"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// A MutatingAdmissionPolicy can report a passing mutation and an error from
// different bindings. The mutation must still be written out once, and an
// error-only response must not claim a mutation was applied.
func Test_ApplyPoliciesOnResource_mutatingAdmissionPolicyOutput(t *testing.T) {
	badBinding := admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "bad"},
		Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
			ParamRef: &admissionregistrationv1beta1.ParamRef{
				Selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      "env",
					Operator: "Bogus",
					Values:   []string{"prod"},
				}}},
			},
		},
	}
	goodBinding := admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "good"},
	}

	tests := []struct {
		name        string
		bindings    []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding
		want        ResultCounts
		wantWritten bool
	}{{
		name:        "passing mutation next to an error is written",
		bindings:    []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{badBinding, goodBinding},
		want:        ResultCounts{Pass: 1, Error: 1},
		wantWritten: true,
	}, {
		name:        "error only response claims no mutation",
		bindings:    []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{badBinding},
		want:        ResultCounts{Error: 1},
		wantWritten: false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources, err := resource.GetUnstructuredResources([]byte(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"workload","namespace":"default"},"spec":{"replicas":1}}`))
			assert.NilError(t, err)

			policy := admissionregistrationv1beta1.MutatingAdmissionPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "label-deployment"},
				Spec: admissionregistrationv1beta1.MutatingAdmissionPolicySpec{
					ParamKind: &admissionregistrationv1beta1.ParamKind{APIVersion: "v1", Kind: "ConfigMap"},
					Mutations: []admissionregistrationv1beta1.Mutation{{
						PatchType: admissionregistrationv1beta1.PatchTypeApplyConfiguration,
						ApplyConfiguration: &admissionregistrationv1beta1.ApplyConfiguration{
							Expression: `Object{ metadata: Object.metadata{ labels: {"applied": "yes"} } }`,
						},
					}},
				},
			}
			bindings := make([]admissionregistrationv1beta1.MutatingAdmissionPolicyBinding, len(tt.bindings))
			copy(bindings, tt.bindings)
			for i := range bindings {
				bindings[i].Spec.PolicyName = policy.Name
			}

			outputFile := filepath.Join(t.TempDir(), "mutated.yaml")
			assert.NilError(t, os.WriteFile(outputFile, nil, 0o600))
			out := &bytes.Buffer{}
			rc := &ResultCounts{}
			mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "apps", Version: "v1"}})
			mapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
			p := PolicyProcessor{
				Store:                           &store.Store{},
				MutatingAdmissionPolicies:       []admissionregistrationv1beta1.MutatingAdmissionPolicy{policy},
				MutatingAdmissionPolicyBindings: bindings,
				Resource:                        *resources[0],
				Variables:                       &variables.Variables{},
				Rc:                              rc,
				Out:                             out,
				MutateLogPath:                   outputFile,
				MutateLogPathIsDir:              false,
				RESTMapper:                      mapper,
			}
			_, err = p.ApplyPoliciesOnResource()
			assert.NilError(t, err)

			assert.DeepEqual(t, *rc, tt.want)
			wantSuccess := 0
			if tt.wantWritten {
				wantSuccess = 1
			}
			assert.Equal(t, strings.Count(out.String(), "Mutation has been applied successfully"), wantSuccess)
			written, err := os.ReadFile(outputFile)
			assert.NilError(t, err)
			if tt.wantWritten {
				assert.Equal(t, strings.Count(string(written), "kind: Deployment"), 1)
			} else {
				assert.Equal(t, len(written), 0)
			}
		})
	}
}

// A later binding that skips does not apply a patch, but a response can still
// carry the mutation an earlier binding applied through resource comparison.
func TestMutationApplied(t *testing.T) {
	original := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"name": "workload"},
	}}
	changed := original.DeepCopy()
	changed.SetLabels(map[string]string{"applied": "yes"})

	tests := []struct {
		name     string
		original unstructured.Unstructured
		patched  unstructured.Unstructured
		rules    []engineapi.RuleResponse
		want     bool
	}{{
		name:     "unchanged resource with an error only",
		original: original,
		patched:  original,
		rules:    []engineapi.RuleResponse{*engineapi.RuleError("map", engineapi.Mutation, "failed", nil, nil)},
		want:     false,
	}, {
		name:     "pass rule",
		original: original,
		patched:  original,
		rules:    []engineapi.RuleResponse{*engineapi.RulePass("map", engineapi.Mutation, "", nil)},
		want:     true,
	}, {
		name:     "changed resource without a pass rule",
		original: original,
		patched:  *changed,
		rules:    []engineapi.RuleResponse{*engineapi.RuleError("map", engineapi.Mutation, "failed", nil, nil)},
		want:     true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := engineapi.NewEngineResponse(tt.original, nil, nil)
			response.PatchedResource = tt.patched
			response.PolicyResponse.Rules = tt.rules
			assert.Equal(t, mutationApplied(response, tt.original), tt.want)
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
