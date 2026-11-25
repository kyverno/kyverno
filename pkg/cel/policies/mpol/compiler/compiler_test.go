package compiler

import (
	"context"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
)

type policyWithoutSpec struct {
	v1beta1.MutatingPolicyLike
}

func (policyWithoutSpec) GetSpec() *v1beta1.MutatingPolicySpec { return nil }

func TestCompileMissingPolicy(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		policy v1beta1.MutatingPolicyLike
	}{
		{"nil", nil},
		{"typed nil", (*v1beta1.MutatingPolicy)(nil)},
		{"namespaced typed nil", (*v1beta1.NamespacedMutatingPolicy)(nil)},
		{"missing spec", policyWithoutSpec{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compiled, errs := NewCompiler().Compile(tc.policy, nil)
			assert.Nil(t, compiled)
			if assert.Len(t, errs, 1) {
				assert.Equal(t, field.ErrorTypeRequired, errs[0].Type)
			}
		})
	}
}

func TestCompileJSONApplyConfigurationIsUnsupported(t *testing.T) {
	policy := &v1beta1.MutatingPolicy{
		Spec: v1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &v1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
				ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
					Expression: `Object{}`,
				},
			}},
		},
	}
	compiled, errs := NewCompiler().Compile(policy, nil)
	assert.Nil(t, compiled)
	if assert.NotEmpty(t, errs) {
		assert.Contains(t, errs[0].Error(), "ApplyConfiguration mutations are not supported in JSON mode")
	}
}

func TestCompile(t *testing.T) {
	tests := []struct {
		name    string
		pol     *v1beta1.MutatingPolicy
		polex   []*v1beta1.PolicyException
		wantErr bool
	}{
		{
			name: "valid applyConfiguration mutation",
			pol: &v1beta1.MutatingPolicy{
				Spec: v1beta1.MutatingPolicySpec{
					Mutations: []admissionregistrationv1alpha1.Mutation{{
						PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: `Object{spec: Object.spec{containers: [Object.spec.containers{name: "nginx"}]}}`,
						},
					}},
				},
			},
			polex:   nil,
			wantErr: false,
		},
		{
			name: "invalid-expression in exception",
			pol: &v1beta1.MutatingPolicy{
				Spec: v1beta1.MutatingPolicySpec{},
			},
			polex: []*v1beta1.PolicyException{{
				Spec: v1beta1.PolicyExceptionSpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{{Expression: "invalid && expression"}},
				},
			}},
			wantErr: true,
		},
		{
			name: "valid jsonpatch mutation",
			pol: &v1beta1.MutatingPolicy{
				Spec: v1beta1.MutatingPolicySpec{
					Mutations: []admissionregistrationv1alpha1.Mutation{{
						PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
						JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
							Expression: `[JSONPatch{op: "add", path: "/spec/restartPolicy", value: "Always"}]`,
						},
					}},
				},
			},
			polex:   nil,
			wantErr: false,
		},
		{
			name: "policy with variables",
			pol: &v1beta1.MutatingPolicy{
				Spec: v1beta1.MutatingPolicySpec{
					Variables: []admissionregistrationv1.Variable{{
						Name:       "foo",
						Expression: "object.metadata.name",
					}},
					Mutations: []admissionregistrationv1alpha1.Mutation{{
						PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: `Object{}`,
						},
					}},
				},
			},
			polex:   nil,
			wantErr: false,
		},
		{
			name: "valid matchCondition expression",
			pol: &v1beta1.MutatingPolicy{
				Spec: v1beta1.MutatingPolicySpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{{
						Name:       "ns-is-test",
						Expression: `true`,
					}},
				},
			},
			polex:   nil,
			wantErr: false,
		},
		{
			name: "invalid matchCondition expression",
			pol: &v1beta1.MutatingPolicy{
				Spec: v1beta1.MutatingPolicySpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{{
						Name:       "ns-is-test",
						Expression: `this is not a cel`,
					}},
				},
			},
			polex:   nil,
			wantErr: true,
		},
		{
			name: "variables allowed in matchConditions when MAP autogen disabled",
			pol: &v1beta1.MutatingPolicy{
				Spec: v1beta1.MutatingPolicySpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{{
						Name:       "uses-variable",
						Expression: `variables.targetName == "test"`,
					}},
					Variables: []admissionregistrationv1.Variable{{
						Name:       "targetName",
						Expression: `"test"`,
					}},
				},
			},
			polex:   nil,
			wantErr: false,
		},
		{
			name: "variables forbidden in matchConditions when MAP autogen enabled",
			pol: &v1beta1.MutatingPolicy{
				Spec: v1beta1.MutatingPolicySpec{
					Variables: []admissionregistrationv1.Variable{{
						Name:       "targetName",
						Expression: `"test"`,
					}},
					MatchConditions: []admissionregistrationv1.MatchCondition{{
						Name:       "uses-variable",
						Expression: `variables.targetName == "test"`,
					}},
					AutogenConfiguration: &v1beta1.MutatingPolicyAutogenConfiguration{
						MutatingAdmissionPolicy: &v1beta1.MAPGenerationConfiguration{Enabled: ptr.To(true)},
					},
				},
			},
			polex:   nil,
			wantErr: true,
		},
		{
			name: "string literals mentioning variables are allowed when MAP autogen enabled",
			pol: &v1beta1.MutatingPolicy{
				Spec: v1beta1.MutatingPolicySpec{
					MatchConditions: []admissionregistrationv1.MatchCondition{{
						Name:       "string-literal",
						Expression: `"variables.targetName" == "variables.targetName"`,
					}},
					AutogenConfiguration: &v1beta1.MutatingPolicyAutogenConfiguration{
						MutatingAdmissionPolicy: &v1beta1.MAPGenerationConfiguration{Enabled: ptr.To(true)},
					},
				},
			},
			polex:   nil,
			wantErr: false,
		},
		{
			name: "variables available in targetMatchConditions",
			pol: &v1beta1.MutatingPolicy{
				Spec: v1beta1.MutatingPolicySpec{
					Variables: []admissionregistrationv1.Variable{{
						Name:       "targetName",
						Expression: `"test"`,
					}},
					TargetMatchConditions: []admissionregistrationv1.MatchCondition{{
						Name:       "uses-variable",
						Expression: `variables.targetName == "test"`,
					}},
				},
			},
			wantErr: false,
		},
		{
			name: "variables available in target expression",
			pol: &v1beta1.MutatingPolicy{
				Spec: v1beta1.MutatingPolicySpec{
					Variables: []admissionregistrationv1.Variable{{
						Name:       "target",
						Expression: `resource.get("v1", "configmaps", "default", "test")`,
					}},
					TargetMatchConstraints: &v1beta1.TargetMatchConstraints{
						Expression: `variables.target`,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid jsonPatch expression",
			pol: &v1beta1.MutatingPolicy{
				Spec: v1beta1.MutatingPolicySpec{
					Mutations: []admissionregistrationv1alpha1.Mutation{{
						PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
						JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
							Expression: `contains(object.metadata.labels) ??
											[
												JSONPatch{
													op: "multiply",
													path: "/metadata/labels/managed",
													value: "true"
												}
											] : [
												JSONPatch{
													op: "add",
													path: "/metadata/labels",
													value: {"managed": "true"}
												}
											]`,
						},
					}},
				},
			},
			polex:   nil,
			wantErr: true,
		},
		{
			name: "invalid jsonPatch expression",
			pol: &v1beta1.MutatingPolicy{
				Spec: v1beta1.MutatingPolicySpec{
					Mutations: []admissionregistrationv1alpha1.Mutation{{
						PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: `invalid{spec: Object.spec{containers: [Object.spec.containers{name: "nginx"}]}}`,
						},
					}},
				},
			},
			polex:   nil,
			wantErr: true,
		},
		{
			name: "valid auditAnnotations",
			pol: &v1beta1.MutatingPolicy{
				Spec: v1beta1.MutatingPolicySpec{
					Mutations: []admissionregistrationv1alpha1.Mutation{{
						PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: `Object{}`,
						},
					}},
					AuditAnnotations: []admissionregistrationv1.AuditAnnotation{{
						Key:             "resource-name",
						ValueExpression: `'name/' + object.metadata.name`,
					}},
				},
			},
			polex:   nil,
			wantErr: false,
		},
		{
			name: "auditAnnotations can reference variables",
			pol: &v1beta1.MutatingPolicy{
				Spec: v1beta1.MutatingPolicySpec{
					Variables: []admissionregistrationv1.Variable{{
						Name:       "resourceName",
						Expression: `object.metadata.name`,
					}},
					AuditAnnotations: []admissionregistrationv1.AuditAnnotation{{
						Key:             "resource-name",
						ValueExpression: `'resource/' + string(variables.resourceName)`,
					}},
				},
			},
			polex:   nil,
			wantErr: false,
		},
		{
			name: "invalid auditAnnotation expression",
			pol: &v1beta1.MutatingPolicy{
				Spec: v1beta1.MutatingPolicySpec{
					AuditAnnotations: []admissionregistrationv1.AuditAnnotation{{
						Key:             "bad",
						ValueExpression: `this is not a cel`,
					}},
				},
			},
			polex:   nil,
			wantErr: true,
		},
	}

	compiler := NewCompiler()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, errs := compiler.Compile(tt.pol, tt.polex)
			if tt.wantErr {
				assert.NotEmpty(t, errs, "expected error but got none")
			} else {
				assert.Empty(t, errs, "expected no error but got some: %v", errs)
				assert.NotNil(t, compiled, "expected compiled policy but got nil")
			}
		})
	}
}

func TestCompileAuditAnnotations(t *testing.T) {
	pol := &v1beta1.MutatingPolicy{
		Spec: v1beta1.MutatingPolicySpec{
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
				ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
					Expression: `Object{}`,
				},
			}},
			AuditAnnotations: []admissionregistrationv1.AuditAnnotation{
				{Key: "resource-name", ValueExpression: `'name/' + object.metadata.name`},
				{Key: "static", ValueExpression: `'value'`},
			},
		},
	}
	compiled, errs := NewCompiler().Compile(pol, nil)
	assert.Empty(t, errs)
	assert.NotNil(t, compiled)
	assert.Len(t, compiled.auditAnnotations, 2)
	assert.Contains(t, compiled.auditAnnotations, "resource-name")
	assert.Contains(t, compiled.auditAnnotations, "static")
}

// TestEvaluateApplyConfigurationMetadataUnderPodTemplate compiles the mutation
// autogen generates for a metadata change on a built-in workload and applies it
// to a Deployment. The ApplyConfiguration patcher checks that every nested
// constructor's type name matches its field path from the root object, so the
// wrapped metadata constructor must be named Object.spec.template.metadata: the
// pod-shaped Object.metadata is rejected once it sits under spec.template.
func TestEvaluateApplyConfigurationMetadataUnderPodTemplate(t *testing.T) {
	ctx := context.TODO()
	tests := []struct {
		name      string
		expr      string
		variables []admissionregistrationv1.Variable
		wantErr   bool
	}{
		{
			name: "template-qualified metadata constructor is accepted",
			expr: `Object{spec: Object.spec{template: Object.spec.template{metadata: Object.spec.template.metadata{labels: {'team': 'dev'}}}}}`,
		},
		{
			// The constructor comes from a variable, which autogen qualifies
			// the same way as the mutation's own constructors.
			name: "template-qualified constructor supplied through a variable is accepted",
			expr: `Object{spec: Object.spec{template: Object.spec.template{metadata: variables.podMetadata}}}`,
			variables: []admissionregistrationv1.Variable{{
				Name:       "podMetadata",
				Expression: `Object.spec.template.metadata{labels: {'team': 'dev'}}`,
			}},
		},
		{
			name: "pod-shaped constructor supplied through a variable is rejected",
			expr: `Object{spec: Object.spec{template: Object.spec.template{metadata: variables.podMetadata}}}`,
			variables: []admissionregistrationv1.Variable{{
				Name:       "podMetadata",
				Expression: `Object.metadata{labels: {'team': 'dev'}}`,
			}},
			wantErr: true,
		},
		{
			name:    "pod-shaped metadata constructor is rejected under spec.template",
			expr:    `Object{spec: Object.spec{template: Object.spec.template{metadata: Object.metadata{labels: {'team': 'dev'}}}}}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pol := &v1beta1.MutatingPolicy{
				Spec: v1beta1.MutatingPolicySpec{
					Variables: tt.variables,
					Mutations: []admissionregistrationv1alpha1.Mutation{{
						PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: tt.expr,
						},
					}},
				},
			}
			compiled, errs := NewCompiler().Compile(pol, nil)
			require.Empty(t, errs)

			res := compiled.Evaluate(ctx, &mockAttributes{}, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}, admissionv1.AdmissionRequest{}, &fakeTCM{}, nil, &libs.FakeContextProvider{})
			require.NotNil(t, res)
			if tt.wantErr {
				require.Error(t, res.Error)
				assert.Contains(t, res.Error.Error(), "type mismatch")
				return
			}
			require.NoError(t, res.Error)

			patched := res.PatchedResource
			require.NotNil(t, patched)
			labels, found, err := unstructured.NestedStringMap(patched.Object, "spec", "template", "metadata", "labels")
			require.NoError(t, err)
			require.True(t, found, "expected the pod template labels to be patched: %v", patched.Object)
			assert.Equal(t, "dev", labels["team"])

			_, deploymentLabelled, _ := unstructured.NestedStringMap(patched.Object, "metadata", "labels")
			assert.False(t, deploymentLabelled, "the Deployment's own labels must not be patched")
		})
	}
}
