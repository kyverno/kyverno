package admissionpolicy

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildMutatingAdmissionPolicyBeta(t *testing.T) {
	mp := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mpol",
			UID:  "test-uid",
			Labels: map[string]string{
				"test-label": "test-value",
			},
		},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						ResourceNames: []string{"test-resource"},
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Create,
								admissionregistrationv1.Update,
							},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "test-condition",
					Expression: "true",
				},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{
				{
					PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
					ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
						Expression: "Object{spec: Object{replicas: 3}}",
					},
				},
			},
			Variables: []admissionregistrationv1.Variable{
				{
					Name:       "test-var",
					Expression: "'test-value'",
				},
			},
		},
	}

	mapol := &admissionregistrationv1beta1.MutatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mpol-test-mpol",
		},
	}

	exceptions := []policiesv1beta1.PolicyException{
		{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{
					{
						Name:       "exception-condition",
						Expression: "object.metadata.name == 'skip'",
					},
				},
			},
		},
	}

	BuildMutatingAdmissionPolicyBeta(mapol, mp, exceptions)

	// Verify owner reference
	assert.Len(t, mapol.OwnerReferences, 1)
	assert.Equal(t, mp.GetName(), mapol.OwnerReferences[0].Name)
	assert.Equal(t, mp.GetUID(), mapol.OwnerReferences[0].UID)

	// Verify match constraints
	assert.NotNil(t, mapol.Spec.MatchConstraints)
	assert.Len(t, mapol.Spec.MatchConstraints.ResourceRules, 1)
	assert.Equal(t, "test-resource", mapol.Spec.MatchConstraints.ResourceRules[0].ResourceNames[0])

	// Verify match conditions (original + negated exceptions)
	assert.Len(t, mapol.Spec.MatchConditions, 2)
	// First condition should be the exception (negated)
	assert.Equal(t, "exception-condition", mapol.Spec.MatchConditions[0].Name)
	assert.Equal(t, "!(object.metadata.name == 'skip')", mapol.Spec.MatchConditions[0].Expression)
	// Second condition should be the original policy condition
	assert.Equal(t, "test-condition", mapol.Spec.MatchConditions[1].Name)
	assert.Equal(t, "true", mapol.Spec.MatchConditions[1].Expression)

	// Verify mutations
	assert.Len(t, mapol.Spec.Mutations, 1)
	assert.Equal(t, admissionregistrationv1beta1.PatchTypeApplyConfiguration, mapol.Spec.Mutations[0].PatchType)

	// Verify variables
	assert.Len(t, mapol.Spec.Variables, 1)
	assert.Equal(t, "test-var", mapol.Spec.Variables[0].Name)

	// Verify labels
	assert.NotNil(t, mapol.Labels)
	assert.Contains(t, mapol.Labels, "app.kubernetes.io/managed-by")
}

func TestBuildMutatingAdmissionPolicyV1(t *testing.T) {
	mp := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mpol",
			UID:  "test-uid",
		},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				}},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "test-condition", Expression: "true"}},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
				ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
					Expression: "Object{spec: Object{replicas: 3}}",
				},
			}},
			Variables: []admissionregistrationv1.Variable{{Name: "test-var", Expression: "'test-value'"}},
		},
	}

	mapol := &admissionregistrationv1.MutatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: "mpol-test-mpol"}}

	BuildMutatingAdmissionPolicyV1(mapol, mp, nil)

	assert.Len(t, mapol.OwnerReferences, 1)
	assert.Equal(t, mp.GetName(), mapol.OwnerReferences[0].Name)
	assert.NotNil(t, mapol.Spec.MatchConstraints)
	assert.Len(t, mapol.Spec.MatchConstraints.ResourceRules, 1)
	assert.Len(t, mapol.Spec.MatchConditions, 1)
	assert.Len(t, mapol.Spec.Mutations, 1)
	assert.Equal(t, admissionregistrationv1.PatchTypeApplyConfiguration, mapol.Spec.Mutations[0].PatchType)
	assert.NotNil(t, mapol.Spec.Mutations[0].ApplyConfiguration)
	assert.Equal(t, "Object{spec: Object{replicas: 3}}", mapol.Spec.Mutations[0].ApplyConfiguration.Expression)
	assert.Equal(t, admissionregistrationv1.ReinvocationPolicyType(mp.Spec.GetReinvocationPolicy()), mapol.Spec.ReinvocationPolicy)
	assert.Len(t, mapol.Spec.Variables, 1)
	assert.Contains(t, mapol.Labels, "app.kubernetes.io/managed-by")
}

func TestBuildMutatingAdmissionPolicyBindingV1(t *testing.T) {
	mp := &policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "test-mpol", UID: "test-uid"}}
	mapbinding := &admissionregistrationv1.MutatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: "mpol-test-mpol-binding"}}

	BuildMutatingAdmissionPolicyBindingV1(mapbinding, mp)

	assert.Len(t, mapbinding.OwnerReferences, 1)
	assert.Equal(t, mp.GetName(), mapbinding.OwnerReferences[0].Name)
	assert.Equal(t, "mpol-test-mpol", mapbinding.Spec.PolicyName)
	assert.Contains(t, mapbinding.Labels, "app.kubernetes.io/managed-by")
}

func TestBuildMutatingAdmissionPolicyBindingBeta(t *testing.T) {
	mp := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mpol",
			UID:  "test-uid",
		},
	}

	mapbinding := &admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mpol-test-mpol-binding",
		},
	}

	BuildMutatingAdmissionPolicyBindingBeta(mapbinding, mp)

	// Verify owner reference
	assert.Len(t, mapbinding.OwnerReferences, 1)
	assert.Equal(t, mp.GetName(), mapbinding.OwnerReferences[0].Name)
	assert.Equal(t, mp.GetUID(), mapbinding.OwnerReferences[0].UID)

	// Verify policy name
	assert.Equal(t, "mpol-test-mpol", mapbinding.Spec.PolicyName)

	// Verify labels
	assert.NotNil(t, mapbinding.Labels)
	assert.Contains(t, mapbinding.Labels, "app.kubernetes.io/managed-by")
}

func TestBuildMutatingAdmissionPolicyBeta_WithFailurePolicy(t *testing.T) {
	failurePolicy := admissionregistrationv1.Fail
	mp := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mpol",
		},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			FailurePolicy: (*admissionregistrationv1.FailurePolicyType)(&failurePolicy),
		},
	}

	mapol := &admissionregistrationv1beta1.MutatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mpol-test-mpol",
		},
	}

	BuildMutatingAdmissionPolicyBeta(mapol, mp, nil)

	// Verify failure policy
	assert.NotNil(t, mapol.Spec.FailurePolicy)
	assert.Equal(t, admissionregistrationv1beta1.Fail, *mapol.Spec.FailurePolicy)
}

func TestBuildMutatingAdmissionPolicyBeta_MutationTypeConversion(t *testing.T) {
	// Test ApplyConfiguration mutation
	mp := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{
				{
					PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
					ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
						Expression: "Object{metadata: Object{labels: {'app': 'test'}}}",
					},
				},
			},
		},
	}

	mapol := &admissionregistrationv1beta1.MutatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
	}

	BuildMutatingAdmissionPolicyBeta(mapol, mp, nil)

	assert.Len(t, mapol.Spec.Mutations, 1)
	assert.Equal(t, admissionregistrationv1beta1.PatchTypeApplyConfiguration, mapol.Spec.Mutations[0].PatchType)
	assert.NotNil(t, mapol.Spec.Mutations[0].ApplyConfiguration)
	assert.Equal(t, "Object{metadata: Object{labels: {'app': 'test'}}}", mapol.Spec.Mutations[0].ApplyConfiguration.Expression)

	// Test JSONPatch mutation
	mp2 := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test2"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{
				{
					PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
					JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
						Expression: "[{'op': 'add', 'path': '/metadata/labels/app', 'value': 'test'}]",
					},
				},
			},
		},
	}

	mapol2 := &admissionregistrationv1beta1.MutatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test2"},
	}

	BuildMutatingAdmissionPolicyBeta(mapol2, mp2, nil)

	assert.Len(t, mapol2.Spec.Mutations, 1)
	assert.Equal(t, admissionregistrationv1beta1.PatchTypeJSONPatch, mapol2.Spec.Mutations[0].PatchType)
	assert.NotNil(t, mapol2.Spec.Mutations[0].JSONPatch)
	assert.Equal(t, "[{'op': 'add', 'path': '/metadata/labels/app', 'value': 'test'}]", mapol2.Spec.Mutations[0].JSONPatch.Expression)
}

// TestGeneratedAdmissionPolicyReportingLabels exercises creation and every reporting
// state transition on the same generated object across all supported API versions.
func TestGeneratedAdmissionPolicyReportingLabels(t *testing.T) {
	t.Parallel()
	states := []struct {
		name              string
		labels            map[string]string
		enabled, disabled bool
	}{
		{name: "absent"},
		{name: "enabled", labels: map[string]string{kyverno.LabelEnableVAPReporting: "true"}, enabled: true},
		{name: "false", labels: map[string]string{kyverno.LabelEnableVAPReporting: "false"}},
		{name: "empty", labels: map[string]string{kyverno.LabelEnableVAPReporting: ""}},
		{name: "disabled", labels: map[string]string{kyverno.LabelExcludeReporting: "false"}, disabled: true},
		{name: "both", labels: map[string]string{kyverno.LabelEnableVAPReporting: "true", kyverno.LabelExcludeReporting: ""}, disabled: true},
	}
	builders := []struct {
		name      string
		newObject func() metav1.Object
		build     func(metav1.Object, map[string]string) error
	}{
		{"clusterpolicy-vap", func() metav1.Object { return &admissionregistrationv1.ValidatingAdmissionPolicy{} }, func(obj metav1.Object, labels map[string]string) error {
			source := &kyvernov1.ClusterPolicy{ObjectMeta: metav1.ObjectMeta{Name: "test", Labels: labels}, Spec: kyvernov1.Spec{Rules: []kyvernov1.Rule{{Name: "test", Validation: &kyvernov1.Validation{CEL: &kyvernov1.CEL{}}}}}}
			return BuildValidatingAdmissionPolicy(nil, obj.(*admissionregistrationv1.ValidatingAdmissionPolicy), engineapi.NewKyvernoPolicy(source), nil)
		}},
		{"vap", func() metav1.Object { return &admissionregistrationv1.ValidatingAdmissionPolicy{} }, func(obj metav1.Object, labels map[string]string) error {
			source := &policiesv1beta1.ValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "test", Labels: labels}, Spec: policiesv1beta1.ValidatingPolicySpec{MatchConstraints: &admissionregistrationv1.MatchResources{}}}
			return BuildValidatingAdmissionPolicy(nil, obj.(*admissionregistrationv1.ValidatingAdmissionPolicy), engineapi.NewValidatingPolicy(source), nil)
		}},
		{"map-alpha", func() metav1.Object { return &admissionregistrationv1alpha1.MutatingAdmissionPolicy{} }, func(obj metav1.Object, labels map[string]string) error {
			BuildMutatingAdmissionPolicy(obj.(*admissionregistrationv1alpha1.MutatingAdmissionPolicy), &policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "test", Labels: labels}, Spec: policiesv1beta1.MutatingPolicySpec{MatchConstraints: &admissionregistrationv1.MatchResources{}}}, nil)
			return nil
		}},
		{"map-beta", func() metav1.Object { return &admissionregistrationv1beta1.MutatingAdmissionPolicy{} }, func(obj metav1.Object, labels map[string]string) error {
			BuildMutatingAdmissionPolicyBeta(obj.(*admissionregistrationv1beta1.MutatingAdmissionPolicy), &policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "test", Labels: labels}, Spec: policiesv1beta1.MutatingPolicySpec{MatchConstraints: &admissionregistrationv1.MatchResources{}}}, nil)
			return nil
		}},
		{"map-v1", func() metav1.Object { return &admissionregistrationv1.MutatingAdmissionPolicy{} }, func(obj metav1.Object, labels map[string]string) error {
			BuildMutatingAdmissionPolicyV1(obj.(*admissionregistrationv1.MutatingAdmissionPolicy), &policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "test", Labels: labels}, Spec: policiesv1beta1.MutatingPolicySpec{MatchConstraints: &admissionregistrationv1.MatchResources{}}}, nil)
			return nil
		}},
	}
	for _, builder := range builders {
		t.Run(builder.name, func(t *testing.T) {
			t.Parallel()
			for _, before := range states {
				for _, after := range states {
					t.Run(before.name+"-to-"+after.name, func(t *testing.T) {
						t.Parallel()
						obj := builder.newObject()
						obj.SetLabels(map[string]string{"unrelated": "preserved"})
						require.NoError(t, builder.build(obj, before.labels))
						require.NoError(t, builder.build(obj, after.labels))
						labels := obj.GetLabels()
						_, enabled := labels[kyverno.LabelEnableVAPReporting]
						_, disabled := labels[kyverno.LabelExcludeReporting]
						assert.Equal(t, after.enabled, enabled)
						assert.Equal(t, after.disabled, disabled)
						if enabled {
							assert.Equal(t, "true", labels[kyverno.LabelEnableVAPReporting])
						}
						assert.Equal(t, "preserved", labels["unrelated"])
						assert.Equal(t, kyverno.ValueKyvernoApp, labels[kyverno.LabelAppManagedBy])
					})
				}
			}
		})
	}
}
