package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestValidatingPolicy_GetMatchConstraints(t *testing.T) {
	tests := []struct {
		name   string
		policy *ValidatingPolicy
		want   admissionregistrationv1.MatchResources
	}{{
		name:   "nil",
		policy: &ValidatingPolicy{},
		want:   admissionregistrationv1.MatchResources{},
	}, {
		name: "not nil",
		policy: &ValidatingPolicy{
			Spec: ValidatingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{},
			},
		},
		want: admissionregistrationv1.MatchResources{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.GetMatchConstraints()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidatingPolicy_GetMatchConditions(t *testing.T) {
	tests := []struct {
		name   string
		policy *ValidatingPolicy
		want   []admissionregistrationv1.MatchCondition
	}{{
		name:   "nil",
		policy: &ValidatingPolicy{},
		want:   nil,
	}, {
		name: "empty",
		policy: &ValidatingPolicy{
			Spec: ValidatingPolicySpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{},
			},
		},
		want: []admissionregistrationv1.MatchCondition{},
	}, {
		name: "not empty",
		policy: &ValidatingPolicy{
			Spec: ValidatingPolicySpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{{
					Name:       "dummy",
					Expression: "expression",
				}},
			},
		},
		want: []admissionregistrationv1.MatchCondition{{
			Name:       "dummy",
			Expression: "expression",
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.GetMatchConditions()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidatingPolicy_GetFailurePolicy(t *testing.T) {
	tests := []struct {
		name   string
		policy *ValidatingPolicy
		want   admissionregistrationv1.FailurePolicyType
	}{{
		name:   "nil",
		policy: &ValidatingPolicy{},
		want:   admissionregistrationv1.Fail,
	}, {
		name: "fail",
		policy: &ValidatingPolicy{
			Spec: ValidatingPolicySpec{
				FailurePolicy: ptr.To(admissionregistrationv1.Fail),
			},
		},
		want: admissionregistrationv1.Fail,
	}, {
		name: "ignore",
		policy: &ValidatingPolicy{
			Spec: ValidatingPolicySpec{
				FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
			},
		},
		want: admissionregistrationv1.Ignore,
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.GetFailurePolicy()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidatingPolicy_GetWebhookConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		policy *ValidatingPolicy
		want   *WebhookConfiguration
	}{{
		name:   "nil",
		policy: &ValidatingPolicy{},
		want:   nil,
	}, {
		name: "fail",
		policy: &ValidatingPolicy{
			Spec: ValidatingPolicySpec{
				WebhookConfiguration: &WebhookConfiguration{},
			},
		},
		want: &WebhookConfiguration{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.GetWebhookConfiguration()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidatingPolicy_GetVariables(t *testing.T) {
	tests := []struct {
		name   string
		policy *ValidatingPolicy
		want   []admissionregistrationv1.Variable
	}{{
		name:   "nil",
		policy: &ValidatingPolicy{},
		want:   nil,
	}, {
		name: "empty",
		policy: &ValidatingPolicy{
			Spec: ValidatingPolicySpec{
				Variables: []admissionregistrationv1.Variable{},
			},
		},
		want: []admissionregistrationv1.Variable{},
	}, {
		name: "not empty",
		policy: &ValidatingPolicy{
			Spec: ValidatingPolicySpec{
				Variables: []admissionregistrationv1.Variable{{
					Name:       "dummy",
					Expression: "expression",
				}},
			},
		},
		want: []admissionregistrationv1.Variable{{
			Name:       "dummy",
			Expression: "expression",
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.GetVariables()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidatingPolicy_GetSpec(t *testing.T) {
	tests := []struct {
		name   string
		policy *ValidatingPolicy
		want   *ValidatingPolicySpec
	}{{
		name: "empty",
		policy: &ValidatingPolicy{
			Spec: ValidatingPolicySpec{
				Variables: []admissionregistrationv1.Variable{},
			},
		},
		want: &ValidatingPolicySpec{
			Variables: []admissionregistrationv1.Variable{},
		},
	}, {
		name: "not empty",
		policy: &ValidatingPolicy{
			Spec: ValidatingPolicySpec{
				Variables: []admissionregistrationv1.Variable{{
					Name:       "dummy",
					Expression: "expression",
				}},
			},
		},
		want: &ValidatingPolicySpec{
			Variables: []admissionregistrationv1.Variable{{
				Name:       "dummy",
				Expression: "expression",
			}},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.GetSpec()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidatingPolicy_GetStatus(t *testing.T) {
	tests := []struct {
		name   string
		policy *ValidatingPolicy
		want   *ValidatingPolicyStatus
	}{{
		policy: &ValidatingPolicy{},
		want:   &ValidatingPolicyStatus{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.GetStatus()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidatingPolicy_GetKind(t *testing.T) {
	tests := []struct {
		name   string
		policy *ValidatingPolicy
		want   string
	}{{
		name:   "not set",
		policy: &ValidatingPolicy{},
		want:   "ValidatingPolicy",
	}, {
		name: "set",
		policy: &ValidatingPolicy{
			TypeMeta: v1.TypeMeta{
				Kind: "Foo",
			},
		},
		want: "ValidatingPolicy",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.GetKind()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidatingPolicySpec_ValidationActions(t *testing.T) {
	tests := []struct {
		name   string
		policy *ValidatingPolicy
		want   []admissionregistrationv1.ValidationAction
	}{{
		name:   "nil",
		policy: &ValidatingPolicy{},
		want:   []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
	}, {
		name:   "deny",
		policy: &ValidatingPolicy{Spec: ValidatingPolicySpec{ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny}}},
		want:   []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
	}, {
		name:   "warn",
		policy: &ValidatingPolicy{Spec: ValidatingPolicySpec{ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Warn}}},
		want:   []admissionregistrationv1.ValidationAction{admissionregistrationv1.Warn},
	}, {
		name:   "audit",
		policy: &ValidatingPolicy{Spec: ValidatingPolicySpec{ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit}}},
		want:   []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit},
	}, {
		name:   "multiple",
		policy: &ValidatingPolicy{Spec: ValidatingPolicySpec{ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit, admissionregistrationv1.Warn}}},
		want:   []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit, admissionregistrationv1.Warn},
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.Spec.ValidationActions()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidatingPolicy_BackgroundEnabled(t *testing.T) {
	tests := []struct {
		name   string
		policy *ValidatingPolicy
		want   bool
	}{{
		name:   "nil",
		policy: &ValidatingPolicy{},
		want:   true,
	}, {
		name: "true",
		policy: &ValidatingPolicy{
			Spec: ValidatingPolicySpec{
				EvaluationConfiguration: &EvaluationConfiguration{
					Background: &BackgroundConfiguration{
						Enabled: ptr.To(true),
					},
				},
			},
		},
		want: true,
	}, {
		name: "false",
		policy: &ValidatingPolicy{
			Spec: ValidatingPolicySpec{
				EvaluationConfiguration: &EvaluationConfiguration{
					Background: &BackgroundConfiguration{
						Enabled: ptr.To(false),
					},
				},
			},
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.BackgroundEnabled()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidatingPolicySpec_GenerateValidatingAdmissionPolicyEnabled(t *testing.T) {
	tests := []struct {
		name   string
		policy *ValidatingPolicy
		want   bool
	}{{
		name:   "nil",
		policy: &ValidatingPolicy{},
		want:   false,
	}, {
		name: "nil",
		policy: &ValidatingPolicy{
			Spec: ValidatingPolicySpec{
				AutogenConfiguration: &ValidatingPolicyAutogenConfiguration{},
			},
		},
		want: false,
	}, {
		name: "nil",
		policy: &ValidatingPolicy{
			Spec: ValidatingPolicySpec{
				AutogenConfiguration: &ValidatingPolicyAutogenConfiguration{
					ValidatingAdmissionPolicy: &VapGenerationConfiguration{},
				},
			},
		},
		want: false,
	}, {
		name: "false",
		policy: &ValidatingPolicy{
			Spec: ValidatingPolicySpec{
				AutogenConfiguration: &ValidatingPolicyAutogenConfiguration{
					ValidatingAdmissionPolicy: &VapGenerationConfiguration{
						Enabled: ptr.To(false),
					},
				},
			},
		},
		want: false,
	}, {
		name: "true",
		policy: &ValidatingPolicy{
			Spec: ValidatingPolicySpec{
				AutogenConfiguration: &ValidatingPolicyAutogenConfiguration{
					ValidatingAdmissionPolicy: &VapGenerationConfiguration{
						Enabled: ptr.To(true),
					},
				},
			},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.Spec.GenerateValidatingAdmissionPolicyEnabled()
			assert.Equal(t, tt.want, got)
		})
	}
}
