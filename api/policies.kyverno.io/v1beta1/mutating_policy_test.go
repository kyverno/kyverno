package v1beta1

import (
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func TestMutatingPolicy_GetMatchConstraints(t *testing.T) {
	tests := []struct {
		name     string
		policy   MutatingPolicy
		expected admissionregistrationv1.MatchResources
	}{
		{
			name: "nil match constraints",
			policy: MutatingPolicy{
				Spec: MutatingPolicySpec{},
			},
			expected: admissionregistrationv1.MatchResources{},
		},
		{
			name: "with match constraints",
			policy: MutatingPolicy{
				Spec: MutatingPolicySpec{
					MatchConstraints: &admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
							{
								RuleWithOperations: admissionregistrationv1.RuleWithOperations{
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
									Rule: admissionregistrationv1.Rule{
										APIGroups:   []string{"apps"},
										APIVersions: []string{"v1"},
										Resources:   []string{"deployments"},
									},
								},
							},
						},
					},
				},
			},
			expected: admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"deployments"},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.policy.GetMatchConstraints()
			if len(result.ResourceRules) != len(tt.expected.ResourceRules) {
				t.Errorf("GetMatchConstraints() ResourceRules length = %v, want %v", len(result.ResourceRules), len(tt.expected.ResourceRules))
			}
		})
	}
}

func TestMutatingPolicy_GetTargetMatchConstraints(t *testing.T) {
	tests := []struct {
		name     string
		policy   MutatingPolicy
		expected admissionregistrationv1.MatchResources
	}{
		{
			name: "nil target match constraints",
			policy: MutatingPolicy{
				Spec: MutatingPolicySpec{},
			},
			expected: admissionregistrationv1.MatchResources{},
		},
		{
			name: "with target match constraints",
			policy: MutatingPolicy{
				Spec: MutatingPolicySpec{
					TargetMatchConstraints: &admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
							{
								RuleWithOperations: admissionregistrationv1.RuleWithOperations{
									Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update},
									Rule: admissionregistrationv1.Rule{
										APIGroups:   []string{"apps"},
										APIVersions: []string{"v1"},
										Resources:   []string{"deployments"},
									},
								},
							},
						},
					},
				},
			},
			expected: admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"deployments"},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.policy.GetTargetMatchConstraints()
			if len(result.ResourceRules) != len(tt.expected.ResourceRules) {
				t.Errorf("GetTargetMatchConstraints() ResourceRules length = %v, want %v", len(result.ResourceRules), len(tt.expected.ResourceRules))
			}
		})
	}
}

func TestMutatingPolicy_GetFailurePolicy(t *testing.T) {
	tests := []struct {
		name     string
		policy   MutatingPolicy
		expected admissionregistrationv1.FailurePolicyType
	}{
		{
			name: "nil failure policy",
			policy: MutatingPolicy{
				Spec: MutatingPolicySpec{},
			},
			expected: admissionregistrationv1.Fail,
		},
		{
			name: "with failure policy ignore",
			policy: MutatingPolicy{
				Spec: MutatingPolicySpec{
					FailurePolicy: &[]admissionregistrationv1.FailurePolicyType{admissionregistrationv1.Ignore}[0],
				},
			},
			expected: admissionregistrationv1.Ignore,
		},
		{
			name: "with failure policy fail",
			policy: MutatingPolicy{
				Spec: MutatingPolicySpec{
					FailurePolicy: &[]admissionregistrationv1.FailurePolicyType{admissionregistrationv1.Fail}[0],
				},
			},
			expected: admissionregistrationv1.Fail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.policy.GetFailurePolicy()
			if result != tt.expected {
				t.Errorf("GetFailurePolicy() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMutatingPolicy_GetKind(t *testing.T) {
	policy := MutatingPolicy{}
	expected := "MutatingPolicy"
	result := policy.GetKind()
	if result != expected {
		t.Errorf("GetKind() = %v, want %v", result, expected)
	}
}

func TestMutatingPolicySpec_GetReinvocationPolicy(t *testing.T) {
	tests := []struct {
		name     string
		spec     MutatingPolicySpec
		expected admissionregistrationv1.ReinvocationPolicyType
	}{
		{
			name:     "empty reinvocation policy",
			spec:     MutatingPolicySpec{},
			expected: admissionregistrationv1.NeverReinvocationPolicy,
		},
		{
			name: "with never reinvocation policy",
			spec: MutatingPolicySpec{
				ReinvocationPolicy: admissionregistrationv1.NeverReinvocationPolicy,
			},
			expected: admissionregistrationv1.NeverReinvocationPolicy,
		},
		{
			name: "with if needed reinvocation policy",
			spec: MutatingPolicySpec{
				ReinvocationPolicy: admissionregistrationv1.IfNeededReinvocationPolicy,
			},
			expected: admissionregistrationv1.IfNeededReinvocationPolicy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.spec.GetReinvocationPolicy()
			if result != tt.expected {
				t.Errorf("GetReinvocationPolicy() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMutatingPolicySpec_GenerateMutatingAdmissionPolicyEnabled(t *testing.T) {
	tests := []struct {
		name     string
		spec     MutatingPolicySpec
		expected bool
	}{
		{
			name:     "nil autogen configuration",
			spec:     MutatingPolicySpec{},
			expected: false,
		},
		{
			name: "nil mutating admission policy configuration",
			spec: MutatingPolicySpec{
				AutogenConfiguration: &MutatingPolicyAutogenConfiguration{},
			},
			expected: false,
		},
		{
			name: "nil enabled field",
			spec: MutatingPolicySpec{
				AutogenConfiguration: &MutatingPolicyAutogenConfiguration{
					MutatingAdmissionPolicy: &MAPGenerationConfiguration{},
				},
			},
			expected: false,
		},
		{
			name: "enabled false",
			spec: MutatingPolicySpec{
				AutogenConfiguration: &MutatingPolicyAutogenConfiguration{
					MutatingAdmissionPolicy: &MAPGenerationConfiguration{
						Enabled: &[]bool{false}[0],
					},
				},
			},
			expected: false,
		},
		{
			name: "enabled true",
			spec: MutatingPolicySpec{
				AutogenConfiguration: &MutatingPolicyAutogenConfiguration{
					MutatingAdmissionPolicy: &MAPGenerationConfiguration{
						Enabled: &[]bool{true}[0],
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.spec.GenerateMutatingAdmissionPolicyEnabled()
			if result != tt.expected {
				t.Errorf("GenerateMutatingAdmissionPolicyEnabled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMutatingPolicySpec_AdmissionEnabled(t *testing.T) {
	tests := []struct {
		name     string
		spec     MutatingPolicySpec
		expected bool
	}{
		{
			name:     "nil evaluation configuration",
			spec:     MutatingPolicySpec{},
			expected: true,
		},
		{
			name: "nil admission configuration",
			spec: MutatingPolicySpec{
				EvaluationConfiguration: &MutatingPolicyEvaluationConfiguration{},
			},
			expected: true,
		},
		{
			name: "nil enabled field",
			spec: MutatingPolicySpec{
				EvaluationConfiguration: &MutatingPolicyEvaluationConfiguration{
					Admission: &AdmissionConfiguration{},
				},
			},
			expected: true,
		},
		{
			name: "enabled false",
			spec: MutatingPolicySpec{
				EvaluationConfiguration: &MutatingPolicyEvaluationConfiguration{
					Admission: &AdmissionConfiguration{
						Enabled: &[]bool{false}[0],
					},
				},
			},
			expected: false,
		},
		{
			name: "enabled true",
			spec: MutatingPolicySpec{
				EvaluationConfiguration: &MutatingPolicyEvaluationConfiguration{
					Admission: &AdmissionConfiguration{
						Enabled: &[]bool{true}[0],
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.spec.AdmissionEnabled()
			if result != tt.expected {
				t.Errorf("AdmissionEnabled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMutatingPolicySpec_BackgroundEnabled(t *testing.T) {
	tests := []struct {
		name     string
		spec     MutatingPolicySpec
		expected bool
	}{
		{
			name:     "nil evaluation configuration",
			spec:     MutatingPolicySpec{},
			expected: true,
		},
		{
			name: "nil background configuration",
			spec: MutatingPolicySpec{
				EvaluationConfiguration: &MutatingPolicyEvaluationConfiguration{},
			},
			expected: true,
		},
		{
			name: "nil enabled field",
			spec: MutatingPolicySpec{
				EvaluationConfiguration: &MutatingPolicyEvaluationConfiguration{
					Background: &BackgroundConfiguration{},
				},
			},
			expected: true,
		},
		{
			name: "enabled false",
			spec: MutatingPolicySpec{
				EvaluationConfiguration: &MutatingPolicyEvaluationConfiguration{
					Background: &BackgroundConfiguration{
						Enabled: &[]bool{false}[0],
					},
				},
			},
			expected: false,
		},
		{
			name: "enabled true",
			spec: MutatingPolicySpec{
				EvaluationConfiguration: &MutatingPolicyEvaluationConfiguration{
					Background: &BackgroundConfiguration{
						Enabled: &[]bool{true}[0],
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.spec.BackgroundEnabled()
			if result != tt.expected {
				t.Errorf("BackgroundEnabled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMutatingPolicySpec_EvaluationMode(t *testing.T) {
	tests := []struct {
		name     string
		spec     MutatingPolicySpec
		expected EvaluationMode
	}{
		{
			name:     "nil evaluation configuration",
			spec:     MutatingPolicySpec{},
			expected: EvaluationModeKubernetes,
		},
		{
			name: "empty mode",
			spec: MutatingPolicySpec{
				EvaluationConfiguration: &MutatingPolicyEvaluationConfiguration{},
			},
			expected: EvaluationModeKubernetes,
		},
		{
			name: "kubernetes mode",
			spec: MutatingPolicySpec{
				EvaluationConfiguration: &MutatingPolicyEvaluationConfiguration{
					Mode: EvaluationModeKubernetes,
				},
			},
			expected: EvaluationModeKubernetes,
		},
		{
			name: "json mode",
			spec: MutatingPolicySpec{
				EvaluationConfiguration: &MutatingPolicyEvaluationConfiguration{
					Mode: EvaluationModeJSON,
				},
			},
			expected: EvaluationModeJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.spec.EvaluationMode()
			if result != tt.expected {
				t.Errorf("EvaluationMode() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMutatingPolicySpec_MutateExistingEnabled(t *testing.T) {
	tests := []struct {
		name     string
		spec     MutatingPolicySpec
		expected bool
	}{
		{
			name:     "nil evaluation configuration",
			spec:     MutatingPolicySpec{},
			expected: false,
		},
		{
			name: "nil mutate existing configuration",
			spec: MutatingPolicySpec{
				EvaluationConfiguration: &MutatingPolicyEvaluationConfiguration{},
			},
			expected: false,
		},
		{
			name: "nil enabled field",
			spec: MutatingPolicySpec{
				EvaluationConfiguration: &MutatingPolicyEvaluationConfiguration{
					MutateExistingConfiguration: &MutateExistingConfiguration{},
				},
			},
			expected: false,
		},
		{
			name: "enabled false",
			spec: MutatingPolicySpec{
				EvaluationConfiguration: &MutatingPolicyEvaluationConfiguration{
					MutateExistingConfiguration: &MutateExistingConfiguration{
						Enabled: &[]bool{false}[0],
					},
				},
			},
			expected: false,
		},
		{
			name: "enabled true",
			spec: MutatingPolicySpec{
				EvaluationConfiguration: &MutatingPolicyEvaluationConfiguration{
					MutateExistingConfiguration: &MutateExistingConfiguration{
						Enabled: &[]bool{true}[0],
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.spec.MutateExistingEnabled()
			if result != tt.expected {
				t.Errorf("MutateExistingEnabled() = %v, want %v", result, tt.expected)
			}
		})
	}
}
