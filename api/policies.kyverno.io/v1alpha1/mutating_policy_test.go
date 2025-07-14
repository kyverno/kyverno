package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetMatchConstraints(t *testing.T) {
	t.Run("returns empty when MatchConstraints is nil", func(t *testing.T) {
		mpol := MutatingPolicy{
			Spec: MutatingPolicySpec{},
		}

		result := mpol.GetMatchConstraints()

		assert.Nil(t, result.NamespaceSelector, "NamespaceSelector should be nil")
		assert.Nil(t, result.ObjectSelector, "ObjectSelector should be nil")
		assert.Nil(t, result.MatchPolicy, "MatchPolicy should be nil")
		assert.Empty(t, result.ResourceRules, "ResourceRules should be empty")
		assert.Empty(t, result.ExcludeResourceRules, "ExcludeResourceRules should be empty")
	})

	t.Run("returns copied MatchConstraints", func(t *testing.T) {
		mpAlpha := admissionregistrationv1alpha1.Equivalent
		mp := admissionregistrationv1.MatchPolicyType(mpAlpha)

		mpol := MutatingPolicy{
			Spec: MutatingPolicySpec{
				MatchConstraints: &admissionregistrationv1alpha1.MatchResources{
					MatchPolicy: &mpAlpha,
				},
			},
		}

		result := mpol.GetMatchConstraints()

		assert.NotNil(t, result.MatchPolicy, "expected MatchPolicy %s, got %v", mp, result.MatchPolicy)
		assert.Equal(t, *result.MatchPolicy, mp, "expected MatchPolicy %s, got %v", mp, result.MatchPolicy)
	})
}

func TestGetMatchConditions(t *testing.T) {
	condition := admissionregistrationv1alpha1.MatchCondition{
		Name: "test",
	}
	mpol := MutatingPolicy{
		Spec: MutatingPolicySpec{
			MatchConditions: []admissionregistrationv1alpha1.MatchCondition{condition},
		},
	}
	result := mpol.GetMatchConditions()

	assert.Equal(t, len(result), 1, "expected 1 match condition named 'test', got %+v", result)
	assert.Equal(t, result[0].Name, "test", "expected 1 match condition named 'test', got %+v", result)
}

func TestGenerateMutatingAdmissionPolicyEnabled(t *testing.T) {
	t.Run("returns default false if nil", func(t *testing.T) {
		spec := MutatingPolicySpec{}
		assert.False(t, spec.GenerateMutatingAdmissionPolicyEnabled(), "expected false when configuration is nil")
	})

	t.Run("returns false when MutatingAdmissionPolicy is nil", func(t *testing.T) {
		spec := MutatingPolicySpec{
			AutogenConfiguration: &MutatingPolicyAutogenConfiguration{
				MutatingAdmissionPolicy: nil,
			},
		}

		assert.False(t, spec.GenerateMutatingAdmissionPolicyEnabled(), "expected false when MutatingAdmissionPolicy is nil")
	})

	t.Run("returns false when Enabled is nil", func(t *testing.T) {
		spec := MutatingPolicySpec{
			AutogenConfiguration: &MutatingPolicyAutogenConfiguration{
				MutatingAdmissionPolicy: &MAPGenerationConfiguration{
					Enabled: nil,
				},
			},
		}

		assert.False(t, spec.GenerateMutatingAdmissionPolicyEnabled(), "expected false when Enabled is nil")
	})

	t.Run("returns true when explicitly enabled", func(t *testing.T) {
		val := true
		spec := MutatingPolicySpec{
			AutogenConfiguration: &MutatingPolicyAutogenConfiguration{
				MutatingAdmissionPolicy: &MAPGenerationConfiguration{
					Enabled: &val,
				},
			},
		}

		assert.True(t, spec.GenerateMutatingAdmissionPolicyEnabled(), "expected true when enabled is true")
	})
}

func TestGetFailurePolicy(t *testing.T) {
	t.Run("returns default Fail if  nil", func(t *testing.T) {
		mpol := MutatingPolicy{
			Spec: MutatingPolicySpec{},
		}

		assert.Equal(t, mpol.GetFailurePolicy(), admissionregistrationv1.Fail, "expected default failure policy 'Fail'")
	})

	t.Run("returns provided value", func(t *testing.T) {
		val := admissionregistrationv1alpha1.Ignore
		valAlpha := admissionregistrationv1.FailurePolicyType(val)
		mpol := MutatingPolicy{
			Spec: MutatingPolicySpec{
				FailurePolicy: (*admissionregistrationv1alpha1.FailurePolicyType)(&valAlpha),
			},
		}

		assert.Equal(t, mpol.GetFailurePolicy(), valAlpha, "expected %s, got %s", val, mpol.GetFailurePolicy())
	})
}

func TestAdmissionAndBackgroundEnabled(t *testing.T) {
	t.Run("defaults to true if nil", func(t *testing.T) {
		spec := MutatingPolicySpec{}
		assert.True(t, spec.AdmissionEnabled(), "expected AdmissionEnabled to default to true")
		assert.True(t, spec.BackgroundEnabled(), "expected BackgroundEnabled to default to true")
	})

	t.Run("returns set values", func(t *testing.T) {
		admission := false
		existing := true

		spec := MutatingPolicySpec{
			EvaluationConfiguration: &MutatingPolicyEvaluationConfiguration{
				Admission: &AdmissionConfiguration{
					Enabled: &admission,
				},
				MutateExistingConfiguration: &MutateExistingConfiguration{
					Enabled: &existing,
				},
			},
		}

		assert.False(t, spec.AdmissionEnabled(), "expected AdmissionEnabled to be false")
		assert.True(t, spec.BackgroundEnabled(), "expected BackgroundEnabled to be true")
	})
}

func TestGetAndSetMatchConstrainst(t *testing.T) {
	t.Run("returns expected match constraints", func(t *testing.T) {
		mp := admissionregistrationv1.Equivalent
		nsSelector := &metav1.LabelSelector{
			MatchLabels: map[string]string{"env": "prod"},
		}

		objSelector := &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "nginx"},
		}

		input := admissionregistrationv1.MatchResources{
			NamespaceSelector: nsSelector,
			ObjectSelector:    objSelector,
			MatchPolicy:       &mp,
			ExcludeResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
				{
					ResourceNames: []string{"foo"},
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{"CREATE"},
						Rule:       admissionregistrationv1.Rule{},
					},
				},
			},
			ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
				{
					ResourceNames: []string{"bar"},
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{"UPDATE"},
						Rule:       admissionregistrationv1.Rule{},
					},
				},
			},
		}

		var spec MutatingPolicySpec
		spec.SetMatchConstraints(input)
		result := spec.GetMatchConstraints()

		assert.NotNil(t, result.MatchPolicy, "expected MatchPolicy %s, got %+v", mp, result.MatchPolicy)
		assert.Equal(t, *result.MatchPolicy, mp, "expected MatchPolicy %s, got %+v", mp, result.MatchPolicy)
	})

	t.Run("returns nil if no match constraints are provided", func(t *testing.T) {
		var spec MutatingPolicySpec
		result := spec.GetMatchConstraints()
		assert.Nil(t, result.MatchPolicy, "expected nil MatchPolicy")
	})
}

func TestGetWebhookConfiguration(t *testing.T) {
	t.Run("returns nil when WebhookConfiguration is nil", func(t *testing.T) {
		policy := MutatingPolicy{
			Spec: MutatingPolicySpec{
				WebhookConfiguration: nil,
			},
		}
		result := policy.GetWebhookConfiguration()
		assert.Nil(t, result, "expected nil, got %+v", result)
	})

	t.Run("returns non-nil WebhookConfiguration", func(t *testing.T) {
		cfg := &WebhookConfiguration{}
		policy := MutatingPolicy{
			Spec: MutatingPolicySpec{
				WebhookConfiguration: cfg,
			},
		}
		result := policy.GetWebhookConfiguration()
		assert.Equal(t, result, cfg, "expected %+v, got %+v", cfg, result)
	})
}

func TestGetVariables(t *testing.T) {
	t.Run("returns empty slice when Variables is nil", func(t *testing.T) {
		policy := MutatingPolicy{
			Spec: MutatingPolicySpec{
				Variables: nil,
			},
		}
		result := policy.GetVariables()
		assert.Empty(t, result, "expected empty slice, got %v", result)
	})

	t.Run("returns copied slice of variables", func(t *testing.T) {
		policy := MutatingPolicy{
			Spec: MutatingPolicySpec{
				Variables: []admissionregistrationv1alpha1.Variable{
					{
						Name: "foo",
					},
					{
						Name: "bar",
					},
				},
			},
		}
		result := policy.GetVariables()

		assert.Equal(t, len(result), 2, "expected 2 variables, got %d", len(result))
		assert.Equal(t, "foo", result[0].Name, "unexpected values: %+v", result)
		assert.Equal(t, "bar", result[1].Name, "unexpected values: %+v", result)
	})
}

func TestGetReinvocationPolicy(t *testing.T) {
	t.Run("returns default when ReinvocationPolicy is empty", func(t *testing.T) {
		spec := MutatingPolicySpec{
			ReinvocationPolicy: "",
		}
		expected := admissionregistrationv1alpha1.NeverReinvocationPolicy
		result := spec.GetReinvocationPolicy()
		assert.Equal(t, result, expected, "expected %s, got %s", expected, result)
	})

	t.Run("returns explicitly set ReinvocationPolicy", func(t *testing.T) {
		expected := admissionregistrationv1alpha1.IfNeededReinvocationPolicy
		spec := MutatingPolicySpec{
			ReinvocationPolicy: expected,
		}
		result := spec.GetReinvocationPolicy()
		assert.Equal(t, result, expected, "expected %s, got %s", expected, result)
	})
}

func TestMutatingPolicy_Getters(t *testing.T) {
	t.Run("GetStatus returns pointer to Status field", func(t *testing.T) {
		val := true
		expected := MutatingPolicyStatus{
			ConditionStatus: ConditionStatus{
				Ready: &val,
			},
		}
		policy := MutatingPolicy{
			Status: expected,
		}
		result := policy.GetStatus()
		assert.NotEqual(t, nil, result, "expected non-nil status")
		assert.Equal(t, result.ConditionStatus.Ready, &val, "expected Ready-true, got %+v", result.ConditionStatus.Ready)
	})

	t.Run("GetKind returns 'MutatingPolicy'", func(t *testing.T) {
		policy := MutatingPolicy{}
		result := policy.GetKind()
		expected := "MutatingPolicy"

		assert.Equal(t, expected, result, "expected kind %q, got %q", expected, result)
	})

	t.Run("GetSpec returns pointer to Spec field", func(t *testing.T) {
		expected := MutatingPolicySpec{
			ReinvocationPolicy: "IfNeeded",
		}
		policy := MutatingPolicy{
			Spec: expected,
		}
		result := policy.GetSpec()

		assert.NotEqual(t, result, nil, "expected non-nil spec")
		assert.Equal(t, v1.ReinvocationPolicyType("IfNeeded"), result.ReinvocationPolicy, "expected ReinvocationPolicy 'IfNeeded', got %s", result.ReinvocationPolicy)
	})

	t.Run("GetConditionStatus returns pointer to embedded ConditionStatus", func(t *testing.T) {
		val := true
		status := MutatingPolicyStatus{
			ConditionStatus: ConditionStatus{
				Ready: &val,
			},
		}
		result := status.GetConditionStatus()

		assert.NotNil(t, result, "expected non-nil ConditionStatus")
		assert.Equal(t, result.Ready, &val, "expected Ready=true, got %v", result.Ready)
	})
}
