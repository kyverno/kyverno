package policy

import (
	"context"
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

func TestCompiledMpol_Evaluate(t *testing.T) {
	tests := []struct {
		name           string
		policy         *policiesv1alpha1.MutatingPolicy
		request        *admissionv1.AdmissionRequest
		namespace      *corev1.Namespace
		expectedResult *EvaluationResult
		expectedError  error
	}{
		{
			name: "matcher returns false",
			policy: &policiesv1alpha1.MutatingPolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: policiesv1alpha1.GroupVersion.String(),
					Kind:       "MutatingPolicy",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: policiesv1alpha1.MutatingPolicySpec{
					MutatingAdmissionPolicySpec: admissionregistrationv1alpha1.MutatingAdmissionPolicySpec{
						MatchConstraints: &admissionregistrationv1alpha1.MatchResources{
							ResourceRules: []admissionregistrationv1alpha1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1alpha1.RuleWithOperations{
										Operations: []admissionregistrationv1alpha1.OperationType{admissionregistrationv1alpha1.Create},
										Rule: admissionregistrationv1alpha1.Rule{
											APIGroups:   []string{""},
											APIVersions: []string{"v1"},
											Resources:   []string{"pods"},
										},
									},
								},
							},
						},
						MatchConditions: []admissionregistrationv1alpha1.MatchCondition{
							{
								Name:       "test-match-condition",
								Expression: `has(object.metadata.labels)`,
							},
						},
						Mutations: []admissionregistrationv1alpha1.Mutation{
							{
								PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
								JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
									Expression: `[
										JSONPatch{op: "copy", from: "/metadata/labels/x", path: "/metadata/labels/y"}, 
									]`,
								},
							},
						},
					},
				},
			},
			request: &admissionv1.AdmissionRequest{
				Object: runtime.RawExtension{
					Object: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Pod",
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{
									"x": "1",
								},
							},
						},
					},
				},
				RequestResource: &metav1.GroupVersionResource{
					Group:    "",
					Version:  "v1",
					Resource: "pods",
				},
				Operation: admissionv1.Create,
			},
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
			},
			expectedResult: &EvaluationResult{
				PatchedResource: unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"x": "1",
								"y": "1",
							},
						},
					},
				},
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiler := &compiler{}
			compiledMpol, errList := compiler.CompileMutating(tt.policy, nil)
			assert.Equal(t, errList.ToAggregate(), nil, "error should be nil, got: %v", errList.ToAggregate())

			gvk := tt.request.Object.Object.GetObjectKind().GroupVersionKind()
			gvr := schema.GroupVersionResource(*tt.request.RequestResource)
			attr := admission.NewAttributesRecord(
				tt.request.Object.Object,
				nil,
				gvk,
				tt.request.Namespace,
				tt.request.Name,
				gvr,
				"",
				admission.Operation(tt.request.Operation),
				nil,
				false,
				nil,
			)

			result, err := compiledMpol.Evaluate(
				context.Background(),
				attr,
				tt.request,
				tt.namespace,
				nil,
				0,
			)

			assert.Equal(t, tt.expectedError, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
