package admissionpolicy

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
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
	assert.Equal(t, "test-condition", mapol.Spec.MatchConditions[1].Name)package admissionpolicy

	import (
		"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
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
