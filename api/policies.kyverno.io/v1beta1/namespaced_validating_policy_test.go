package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestNamespacedValidatingPolicy_GetMatchConstraints(t *testing.T) {
	tests := []struct {
		name   string
		policy *NamespacedValidatingPolicy
		want   admissionregistrationv1.MatchResources
	}{{
		name:   "nil",
		policy: &NamespacedValidatingPolicy{},
		want:   admissionregistrationv1.MatchResources{},
	}, {
		name: "not nil",
		policy: &NamespacedValidatingPolicy{
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

func TestNamespacedValidatingPolicy_GetMatchConditions(t *testing.T) {
	tests := []struct {
		name   string
		policy *NamespacedValidatingPolicy
		want   []admissionregistrationv1.MatchCondition
	}{{
		name:   "nil",
		policy: &NamespacedValidatingPolicy{},
		want:   nil,
	}, {
		name: "empty",
		policy: &NamespacedValidatingPolicy{
			Spec: ValidatingPolicySpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{},
			},
		},
		want: []admissionregistrationv1.MatchCondition{},
	}, {
		name: "not empty",
		policy: &NamespacedValidatingPolicy{
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

func TestNamespacedValidatingPolicy_GetFailurePolicy(t *testing.T) {
	tests := []struct {
		name   string
		policy *NamespacedValidatingPolicy
		want   admissionregistrationv1.FailurePolicyType
	}{{
		name:   "nil",
		policy: &NamespacedValidatingPolicy{},
		want:   admissionregistrationv1.Fail,
	}, {
		name: "fail",
		policy: &NamespacedValidatingPolicy{
			Spec: ValidatingPolicySpec{
				FailurePolicy: ptr.To(admissionregistrationv1.Fail),
			},
		},
		want: admissionregistrationv1.Fail,
	}, {
		name: "ignore",
		policy: &NamespacedValidatingPolicy{
			Spec: ValidatingPolicySpec{
				FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
			},
		},
		want: admissionregistrationv1.Ignore,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.GetFailurePolicy()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNamespacedValidatingPolicy_GetWebhookConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		policy *NamespacedValidatingPolicy
		want   *int32
	}{{
		name:   "nil",
		policy: &NamespacedValidatingPolicy{},
		want:   nil,
	}, {
		name: "not nil",
		policy: &NamespacedValidatingPolicy{
			Spec: ValidatingPolicySpec{
				WebhookConfiguration: &WebhookConfiguration{
					TimeoutSeconds: ptr.To[int32](30),
				},
			},
		},
		want: ptr.To[int32](30),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.GetTimeoutSeconds()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNamespacedValidatingPolicy_GetVariables(t *testing.T) {
	tests := []struct {
		name   string
		policy *NamespacedValidatingPolicy
		want   []admissionregistrationv1.Variable
	}{{
		name:   "nil",
		policy: &NamespacedValidatingPolicy{},
		want:   nil,
	}, {
		name: "empty",
		policy: &NamespacedValidatingPolicy{
			Spec: ValidatingPolicySpec{
				Variables: []admissionregistrationv1.Variable{},
			},
		},
		want: []admissionregistrationv1.Variable{},
	}, {
		name: "not empty",
		policy: &NamespacedValidatingPolicy{
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

func TestNamespacedValidatingPolicy_GetSpec(t *testing.T) {
	tests := []struct {
		name   string
		policy *NamespacedValidatingPolicy
		want   *ValidatingPolicySpec
	}{{
		name: "empty",
		policy: &NamespacedValidatingPolicy{
			Spec: ValidatingPolicySpec{
				Variables: []admissionregistrationv1.Variable{},
			},
		},
		want: &ValidatingPolicySpec{
			Variables: []admissionregistrationv1.Variable{},
		},
	}, {
		name: "not empty",
		policy: &NamespacedValidatingPolicy{
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

func TestNamespacedValidatingPolicy_GetStatus(t *testing.T) {
	tests := []struct {
		name   string
		policy *NamespacedValidatingPolicy
		want   *ValidatingPolicyStatus
	}{{
		policy: &NamespacedValidatingPolicy{},
		want:   &ValidatingPolicyStatus{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.GetStatus()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNamespacedValidatingPolicy_GetKind(t *testing.T) {
	tests := []struct {
		name   string
		policy *NamespacedValidatingPolicy
		want   string
	}{{
		name:   "not set",
		policy: &NamespacedValidatingPolicy{},
		want:   "NamespacedValidatingPolicy",
	}, {
		name: "set",
		policy: &NamespacedValidatingPolicy{
			TypeMeta: metav1.TypeMeta{
				Kind: "Foo",
			},
		},
		want: "NamespacedValidatingPolicy",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.GetKind()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNamespacedValidatingPolicySpec_ValidationActions(t *testing.T) {
	tests := []struct {
		name   string
		policy *NamespacedValidatingPolicy
		want   []admissionregistrationv1.ValidationAction
	}{{
		name:   "nil",
		policy: &NamespacedValidatingPolicy{},
		want:   []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
	}, {
		name:   "deny",
		policy: &NamespacedValidatingPolicy{Spec: ValidatingPolicySpec{ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny}}},
		want:   []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
	}, {
		name:   "warn",
		policy: &NamespacedValidatingPolicy{Spec: ValidatingPolicySpec{ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Warn}}},
		want:   []admissionregistrationv1.ValidationAction{admissionregistrationv1.Warn},
	}, {
		name:   "audit",
		policy: &NamespacedValidatingPolicy{Spec: ValidatingPolicySpec{ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit}}},
		want:   []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit},
	}, {
		name:   "multiple",
		policy: &NamespacedValidatingPolicy{Spec: ValidatingPolicySpec{ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit, admissionregistrationv1.Warn}}},
		want:   []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit, admissionregistrationv1.Warn},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.Spec.ValidationActions()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNamespacedValidatingPolicy_BackgroundEnabled(t *testing.T) {
	tests := []struct {
		name   string
		policy *NamespacedValidatingPolicy
		want   bool
	}{{
		name:   "nil",
		policy: &NamespacedValidatingPolicy{},
		want:   true,
	}, {
		name: "true",
		policy: &NamespacedValidatingPolicy{
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
		policy: &NamespacedValidatingPolicy{
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

func TestNamespacedValidatingPolicySpec_GenerateValidatingAdmissionPolicyEnabled(t *testing.T) {
	tests := []struct {
		name   string
		policy *NamespacedValidatingPolicy
		want   bool
	}{{
		name:   "nil",
		policy: &NamespacedValidatingPolicy{},
		want:   false,
	}, {
		name: "nil",
		policy: &NamespacedValidatingPolicy{
			Spec: ValidatingPolicySpec{
				AutogenConfiguration: &ValidatingPolicyAutogenConfiguration{},
			},
		},
		want: false,
	}, {
		name: "nil",
		policy: &NamespacedValidatingPolicy{
			Spec: ValidatingPolicySpec{
				AutogenConfiguration: &ValidatingPolicyAutogenConfiguration{
					ValidatingAdmissionPolicy: &VapGenerationConfiguration{},
				},
			},
		},
		want: false,
	}, {
		name: "false",
		policy: &NamespacedValidatingPolicy{
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
		policy: &NamespacedValidatingPolicy{
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
