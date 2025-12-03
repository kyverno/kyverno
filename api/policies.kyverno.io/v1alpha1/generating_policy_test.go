package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/admissionregistration/v1"
)

var (
	gpol = &GeneratingPolicy{}
)

func TestGetKind(t *testing.T) {
	kind := gpol.GetKind()
	assert.Equal(t, kind, "GeneratingPolicy")
}

func TestGetMatchConstraintsGpol(t *testing.T) {
	t.Run("returns copied MatchConstraints", func(t *testing.T) {
		gpol := &GeneratingPolicy{
			Spec: GeneratingPolicySpec{
				MatchConstraints: &v1.MatchResources{
					ResourceRules: []v1.NamedRuleWithOperations{
						{
							ResourceNames: []string{"pods"},
							RuleWithOperations: v1.RuleWithOperations{
								Operations: []v1.OperationType{"CREATE"},
							},
						},
					},
				},
			},
		}

		res := gpol.GetMatchConstraints()

		assert.NoError(t, nil)
		assert.NotNil(t, res, "res should not be nil")
		assert.Equal(t, res.ResourceRules[0].ResourceNames[0], "pods", "ResourceRules should be 'pods'")
	})

	t.Run("returns empty match constraints", func(t *testing.T) {
		res := gpol.GetMatchConstraints()
		assert.Nil(t, res.NamespaceSelector, "NamespaceSelector should be nil")
		assert.Empty(t, res.ExcludeResourceRules, "ExcludeResourceRules should be empty")
		assert.Nil(t, res.ObjectSelector, "ObjectSelector should be nil")
		assert.Empty(t, res.ResourceRules, "ResourceRules should be empty")
		assert.Nil(t, res.MatchPolicy, "MatchPolicy should be nil")
	})
}

func TestGetMatchConditionsGpol(t *testing.T) {
	gpol := &GeneratingPolicy{
		Spec: GeneratingPolicySpec{
			MatchConditions: []v1.MatchCondition{
				{
					Name:       "test-condition",
					Expression: "true",
				},
			},
		},
	}

	res := gpol.GetMatchConditions()
	assert.NotNil(t, res, "res should not be nil")
	assert.Equal(t, res[0].Name, "test-condition", "name should be 'test-condition")
	assert.Equal(t, res[0].Expression, "true", "expression should be 'true")
}

func TestGetFailurePolicyGpol(t *testing.T) {
	res := gpol.GetFailurePolicy()
	assert.Equal(t, res, v1.Ignore, "result should be 'Ignore")
}

func TestGetWebhookConfigurationGpol(t *testing.T) {
	val := int32(3984)
	gpol := &GeneratingPolicy{
		Spec: GeneratingPolicySpec{
			WebhookConfiguration: &WebhookConfiguration{
				TimeoutSeconds: &val,
			},
		},
	}
	res := gpol.GetWebhookConfiguration()
	assert.Equal(t, res.TimeoutSeconds, &val, "timeout and val should be equal")
}

func TestVariablesGpol(t *testing.T) {
	gpol := &GeneratingPolicy{
		Spec: GeneratingPolicySpec{
			Variables: []v1.Variable{
				{
					Name:       "test-variable",
					Expression: "test-expression",
				},
			},
		},
	}
	res := gpol.GetVariables()
	assert.Equal(t, res[0].Name, "test-variable", "name should be equal")
	assert.Equal(t, res[0].Expression, "test-expression", "expression should be equal")
}

func TestGetSpecGpol(t *testing.T) {
	gpol := &GeneratingPolicy{
		Spec: GeneratingPolicySpec{
			Variables: []v1.Variable{
				{
					Name:       "test-variable",
					Expression: "test-expression",
				},
			},
		},
	}
	res := gpol.GetSpec()
	assert.NotNil(t, res, "res should not be nil")
}

func TestGetStatusGpol(t *testing.T) {
	boolVal := true
	gpol := &GeneratingPolicy{
		Status: GeneratingPolicyStatus{
			ConditionStatus: ConditionStatus{
				Ready: &boolVal,
			},
		},
	}
	res := gpol.GetStatus()
	assert.Equal(t, res.ConditionStatus.Ready, &boolVal, "res should be equal to '&boolVal")
}

func TestOrphanDownStreamOnPolicyDeleteEnabled(t *testing.T) {
	t.Run("empty EvaluationConfiguration", func(t *testing.T) {
		s := GeneratingPolicySpec{
			EvaluationConfiguration: &GeneratingPolicyEvaluationConfiguration{},
		}
		res := s.OrphanDownstreamOnPolicyDeleteEnabled()
		assert.False(t, res)
	})
	t.Run("OrphanDownStreamOnPolicyDelete is nil", func(t *testing.T) {
		s := GeneratingPolicySpec{
			EvaluationConfiguration: &GeneratingPolicyEvaluationConfiguration{
				OrphanDownstreamOnPolicyDelete: &OrphanDownstreamOnPolicyDeleteConfiguration{},
			},
		}
		res := s.OrphanDownstreamOnPolicyDeleteEnabled()
		assert.False(t, res)
	})
	t.Run("OrphanDownStreamOnPolicyDelete.Enabled is nil", func(t *testing.T) {
		s := GeneratingPolicySpec{
			EvaluationConfiguration: &GeneratingPolicyEvaluationConfiguration{
				OrphanDownstreamOnPolicyDelete: &OrphanDownstreamOnPolicyDeleteConfiguration{
					Enabled: nil,
				},
			},
		}
		res := s.OrphanDownstreamOnPolicyDeleteEnabled()
		assert.False(t, res)
	})
	t.Run("complete OrphanDownStreamOnPolicyDelete", func(t *testing.T) {
		boolVal := true
		s := GeneratingPolicySpec{
			EvaluationConfiguration: &GeneratingPolicyEvaluationConfiguration{
				OrphanDownstreamOnPolicyDelete: &OrphanDownstreamOnPolicyDeleteConfiguration{
					Enabled: &boolVal,
				},
			},
		}
		res := s.OrphanDownstreamOnPolicyDeleteEnabled()
		assert.True(t, res)
	})
}

func TestGenerateExistingEnabled(t *testing.T) {
	t.Run("empty EvaluationConfiguration", func(t *testing.T) {
		s := GeneratingPolicySpec{
			EvaluationConfiguration: &GeneratingPolicyEvaluationConfiguration{},
		}
		res := s.GenerateExistingEnabled()
		assert.False(t, res)
	})
	t.Run("GenerateExistingConfiguration is nil", func(t *testing.T) {
		s := GeneratingPolicySpec{
			EvaluationConfiguration: &GeneratingPolicyEvaluationConfiguration{
				GenerateExistingConfiguration: &GenerateExistingConfiguration{},
			},
		}
		res := s.GenerateExistingEnabled()
		assert.False(t, res)
	})
	t.Run("GenerateExistingConfiguration.Enabled is nil", func(t *testing.T) {
		s := GeneratingPolicySpec{
			EvaluationConfiguration: &GeneratingPolicyEvaluationConfiguration{
				GenerateExistingConfiguration: &GenerateExistingConfiguration{
					Enabled: nil,
				},
			},
		}
		res := s.GenerateExistingEnabled()
		assert.False(t, res)
	})
	t.Run("complete GenerateExistingConfiguration", func(t *testing.T) {
		boolVal := true
		s := GeneratingPolicySpec{
			EvaluationConfiguration: &GeneratingPolicyEvaluationConfiguration{
				GenerateExistingConfiguration: &GenerateExistingConfiguration{
					Enabled: &boolVal,
				},
			},
		}
		res := s.GenerateExistingEnabled()
		assert.True(t, res)
	})
}

func TestSynchronizationEnabled(t *testing.T) {
	t.Run("empty EvaluationConfiguration", func(t *testing.T) {
		s := GeneratingPolicySpec{
			EvaluationConfiguration: &GeneratingPolicyEvaluationConfiguration{},
		}
		res := s.SynchronizationEnabled()
		assert.False(t, res)
	})
	t.Run("SynchronizationConfiguration is nil", func(t *testing.T) {
		s := GeneratingPolicySpec{
			EvaluationConfiguration: &GeneratingPolicyEvaluationConfiguration{
				SynchronizationConfiguration: &SynchronizationConfiguration{},
			},
		}
		res := s.SynchronizationEnabled()
		assert.False(t, res)
	})
	t.Run("SynchronizationConfiguration.Enabled is nil", func(t *testing.T) {
		s := GeneratingPolicySpec{
			EvaluationConfiguration: &GeneratingPolicyEvaluationConfiguration{
				SynchronizationConfiguration: &SynchronizationConfiguration{
					Enabled: nil,
				},
			},
		}
		res := s.SynchronizationEnabled()
		assert.False(t, res)
	})
	t.Run("complete SynchronizationEnabled", func(t *testing.T) {
		boolVal := true
		s := GeneratingPolicySpec{
			EvaluationConfiguration: &GeneratingPolicyEvaluationConfiguration{
				SynchronizationConfiguration: &SynchronizationConfiguration{
					Enabled: &boolVal,
				},
			},
		}
		res := s.SynchronizationEnabled()
		assert.True(t, res)
	})
}

func TestAdmissionEnabled(t *testing.T) {
	boolVal := false
	t.Run("empty EvaluationConfiguration", func(t *testing.T) {
		s := GeneratingPolicySpec{
			EvaluationConfiguration: &GeneratingPolicyEvaluationConfiguration{},
		}
		res := s.AdmissionEnabled()
		assert.True(t, res)
	})
	t.Run("AdmissionConfiguration is nil", func(t *testing.T) {
		s := GeneratingPolicySpec{
			EvaluationConfiguration: &GeneratingPolicyEvaluationConfiguration{
				Admission: &AdmissionConfiguration{},
			},
		}
		res := s.AdmissionEnabled()
		assert.True(t, res)
	})
	t.Run("AdmissionConfiguration.Enabled is nil", func(t *testing.T) {
		s := GeneratingPolicySpec{
			EvaluationConfiguration: &GeneratingPolicyEvaluationConfiguration{
				Admission: &AdmissionConfiguration{
					Enabled: nil,
				},
			},
		}
		res := s.AdmissionEnabled()
		assert.True(t, res)
	})
	t.Run("complete AdmissionEnabled", func(t *testing.T) {
		s := GeneratingPolicySpec{
			EvaluationConfiguration: &GeneratingPolicyEvaluationConfiguration{
				SynchronizationConfiguration: &SynchronizationConfiguration{
					Enabled: &boolVal,
				},
			},
		}
		res := s.SynchronizationEnabled()
		assert.False(t, res)
	})
}
