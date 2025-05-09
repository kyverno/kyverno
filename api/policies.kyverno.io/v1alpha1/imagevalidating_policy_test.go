package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestImageValidatingPolicy_GetFailurePolicy(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageValidatingPolicy
		want   admissionregistrationv1.FailurePolicyType
	}{{
		name:   "nil",
		policy: &ImageValidatingPolicy{},
		want:   admissionregistrationv1.Fail,
	}, {
		name: "fail",
		policy: &ImageValidatingPolicy{
			Spec: ImageValidatingPolicySpec{
				FailurePolicy: ptr.To(admissionregistrationv1.Fail),
			},
		},
		want: admissionregistrationv1.Fail,
	}, {
		name: "ignore",
		policy: &ImageValidatingPolicy{
			Spec: ImageValidatingPolicySpec{
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

func TestAttestor_GetKey(t *testing.T) {
	tests := []struct {
		name     string
		attestor Attestor
		want     string
	}{{
		name: "foo",
		attestor: Attestor{
			Name: "foo",
		},
		want: "foo",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.attestor.GetKey()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAttestor_IsCosign(t *testing.T) {
	tests := []struct {
		name     string
		attestor Attestor
		want     bool
	}{{
		name:     "no",
		attestor: Attestor{},
		want:     false,
	}, {
		name: "yes",
		attestor: Attestor{
			Cosign: &Cosign{},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.attestor.IsCosign()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAttestor_IsNotary(t *testing.T) {
	tests := []struct {
		name     string
		attestor Attestor
		want     bool
	}{{
		name:     "no",
		attestor: Attestor{},
		want:     false,
	}, {
		name: "yes",
		attestor: Attestor{
			Notary: &Notary{},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.attestor.IsNotary()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAttestation_GetKey(t *testing.T) {
	tests := []struct {
		name        string
		attestation Attestation
		want        string
	}{{
		name: "foo",
		attestation: Attestation{
			Name: "foo",
		},
		want: "foo",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.attestation.GetKey()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAttestation_IsInToto(t *testing.T) {
	tests := []struct {
		name        string
		attestation Attestation
		want        bool
	}{{
		name:        "no",
		attestation: Attestation{},
		want:        false,
	}, {
		name: "yes",
		attestation: Attestation{
			InToto: &InToto{},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.attestation.IsInToto()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAttestation_IsReferrer(t *testing.T) {
	tests := []struct {
		name        string
		attestation Attestation
		want        bool
	}{{
		name:        "no",
		attestation: Attestation{},
		want:        false,
	}, {
		name: "yes",
		attestation: Attestation{
			Referrer: &Referrer{},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.attestation.IsReferrer()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestImageValidatingPolicySpec_EvaluationMode(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageValidatingPolicySpec
		want   EvaluationMode
	}{{
		name:   "nil",
		policy: &ImageValidatingPolicySpec{},
		want:   EvaluationModeKubernetes,
	}, {
		name: "json",
		policy: &ImageValidatingPolicySpec{
			EvaluationConfiguration: &EvaluationConfiguration{
				Mode: EvaluationModeJSON,
			},
		},
		want: EvaluationModeJSON,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.EvaluationMode()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestImageValidatingPolicy_GetMatchConstraints(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageValidatingPolicy
		want   admissionregistrationv1.MatchResources
	}{{
		name:   "nil",
		policy: &ImageValidatingPolicy{},
		want:   admissionregistrationv1.MatchResources{},
	}, {
		name: "not nil",
		policy: &ImageValidatingPolicy{
			Spec: ImageValidatingPolicySpec{
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

func TestImageValidatingPolicy_GetMatchConditions(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageValidatingPolicy
		want   []admissionregistrationv1.MatchCondition
	}{{
		name:   "nil",
		policy: &ImageValidatingPolicy{},
		want:   nil,
	}, {
		name: "empty",
		policy: &ImageValidatingPolicy{
			Spec: ImageValidatingPolicySpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{},
			},
		},
		want: []admissionregistrationv1.MatchCondition{},
	}, {
		name: "not empty",
		policy: &ImageValidatingPolicy{
			Spec: ImageValidatingPolicySpec{
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

func TestImageValidatingPolicy_GetWebhookConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageValidatingPolicy
		want   *WebhookConfiguration
	}{{
		name:   "nil",
		policy: &ImageValidatingPolicy{},
		want:   nil,
	}, {
		name: "fail",
		policy: &ImageValidatingPolicy{
			Spec: ImageValidatingPolicySpec{
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

func TestImageValidatingPolicy_GetVariables(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageValidatingPolicy
		want   []admissionregistrationv1.Variable
	}{{
		name:   "nil",
		policy: &ImageValidatingPolicy{},
		want:   nil,
	}, {
		name: "empty",
		policy: &ImageValidatingPolicy{
			Spec: ImageValidatingPolicySpec{
				Variables: []admissionregistrationv1.Variable{},
			},
		},
		want: []admissionregistrationv1.Variable{},
	}, {
		name: "not empty",
		policy: &ImageValidatingPolicy{
			Spec: ImageValidatingPolicySpec{
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

func TestImageValidatingPolicy_GetSpec(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageValidatingPolicy
		want   *ImageValidatingPolicySpec
	}{{
		name: "empty",
		policy: &ImageValidatingPolicy{
			Spec: ImageValidatingPolicySpec{
				Variables: []admissionregistrationv1.Variable{},
			},
		},
		want: &ImageValidatingPolicySpec{
			Variables: []admissionregistrationv1.Variable{},
		},
	}, {
		name: "not empty",
		policy: &ImageValidatingPolicy{
			Spec: ImageValidatingPolicySpec{
				Variables: []admissionregistrationv1.Variable{{
					Name:       "dummy",
					Expression: "expression",
				}},
			},
		},
		want: &ImageValidatingPolicySpec{
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

func TestImageValidatingPolicy_GetStatus(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageValidatingPolicy
		want   *ImageValidatingPolicyStatus
	}{{
		policy: &ImageValidatingPolicy{},
		want:   &ImageValidatingPolicyStatus{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.GetStatus()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestImageValidatingPolicy_GetKind(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageValidatingPolicy
		want   string
	}{{
		name:   "not set",
		policy: &ImageValidatingPolicy{},
		want:   "ImageValidatingPolicy",
	}, {
		name: "set",
		policy: &ImageValidatingPolicy{
			TypeMeta: v1.TypeMeta{
				Kind: "Foo",
			},
		},
		want: "ImageValidatingPolicy",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.GetKind()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestImageValidatingPolicy_BackgroundEnabled(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageValidatingPolicy
		want   bool
	}{{
		name:   "nil",
		policy: &ImageValidatingPolicy{},
		want:   true,
	}, {
		name: "true",
		policy: &ImageValidatingPolicy{
			Spec: ImageValidatingPolicySpec{
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
		policy: &ImageValidatingPolicy{
			Spec: ImageValidatingPolicySpec{
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

func TestImageValidatingPolicySpec_AdmissionEnabled(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageValidatingPolicy
		want   bool
	}{{
		name:   "nil",
		policy: &ImageValidatingPolicy{},
		want:   true,
	}, {
		name: "true",
		policy: &ImageValidatingPolicy{
			Spec: ImageValidatingPolicySpec{
				EvaluationConfiguration: &EvaluationConfiguration{
					Admission: &AdmissionConfiguration{
						Enabled: ptr.To(true),
					},
				},
			},
		},
		want: true,
	}, {
		name: "false",
		policy: &ImageValidatingPolicy{
			Spec: ImageValidatingPolicySpec{
				EvaluationConfiguration: &EvaluationConfiguration{
					Admission: &AdmissionConfiguration{
						Enabled: ptr.To(false),
					},
				},
			},
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.Spec.AdmissionEnabled()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestImageValidatingPolicySpec_ValidationActions(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageValidatingPolicy
		want   []admissionregistrationv1.ValidationAction
	}{{
		name:   "nil",
		policy: &ImageValidatingPolicy{},
		want:   []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
	}, {
		name:   "deny",
		policy: &ImageValidatingPolicy{Spec: ImageValidatingPolicySpec{ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny}}},
		want:   []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
	}, {
		name:   "warn",
		policy: &ImageValidatingPolicy{Spec: ImageValidatingPolicySpec{ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Warn}}},
		want:   []admissionregistrationv1.ValidationAction{admissionregistrationv1.Warn},
	}, {
		name:   "audit",
		policy: &ImageValidatingPolicy{Spec: ImageValidatingPolicySpec{ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit}}},
		want:   []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit},
	}, {
		name:   "multiple",
		policy: &ImageValidatingPolicy{Spec: ImageValidatingPolicySpec{ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit, admissionregistrationv1.Warn}}},
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
