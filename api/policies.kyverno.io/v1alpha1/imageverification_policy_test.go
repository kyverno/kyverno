package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestImageVerificationPolicy_GetFailurePolicy(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageVerificationPolicy
		want   admissionregistrationv1.FailurePolicyType
	}{{
		name:   "nil",
		policy: &ImageVerificationPolicy{},
		want:   admissionregistrationv1.Fail,
	}, {
		name: "fail",
		policy: &ImageVerificationPolicy{
			Spec: ImageVerificationPolicySpec{
				FailurePolicy: ptr.To(admissionregistrationv1.Fail),
			},
		},
		want: admissionregistrationv1.Fail,
	}, {
		name: "ignore",
		policy: &ImageVerificationPolicy{
			Spec: ImageVerificationPolicySpec{
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

func TestImageVerificationPolicySpec_EvaluationMode(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageVerificationPolicySpec
		want   EvaluationMode
	}{{
		name:   "nil",
		policy: &ImageVerificationPolicySpec{},
		want:   EvaluationModeKubernetes,
	}, {
		name: "json",
		policy: &ImageVerificationPolicySpec{
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

func TestImageVerificationPolicy_GetMatchConstraints(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageVerificationPolicy
		want   admissionregistrationv1.MatchResources
	}{{
		name:   "nil",
		policy: &ImageVerificationPolicy{},
		want:   admissionregistrationv1.MatchResources{},
	}, {
		name: "not nil",
		policy: &ImageVerificationPolicy{
			Spec: ImageVerificationPolicySpec{
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

func TestImageVerificationPolicy_GetMatchConditions(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageVerificationPolicy
		want   []admissionregistrationv1.MatchCondition
	}{{
		name:   "nil",
		policy: &ImageVerificationPolicy{},
		want:   nil,
	}, {
		name: "empty",
		policy: &ImageVerificationPolicy{
			Spec: ImageVerificationPolicySpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{},
			},
		},
		want: []admissionregistrationv1.MatchCondition{},
	}, {
		name: "not empty",
		policy: &ImageVerificationPolicy{
			Spec: ImageVerificationPolicySpec{
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

func TestImageVerificationPolicy_GetWebhookConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageVerificationPolicy
		want   *WebhookConfiguration
	}{{
		name:   "nil",
		policy: &ImageVerificationPolicy{},
		want:   nil,
	}, {
		name: "fail",
		policy: &ImageVerificationPolicy{
			Spec: ImageVerificationPolicySpec{
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

func TestImageVerificationPolicy_GetVariables(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageVerificationPolicy
		want   []admissionregistrationv1.Variable
	}{{
		name:   "nil",
		policy: &ImageVerificationPolicy{},
		want:   nil,
	}, {
		name: "empty",
		policy: &ImageVerificationPolicy{
			Spec: ImageVerificationPolicySpec{
				Variables: []admissionregistrationv1.Variable{},
			},
		},
		want: []admissionregistrationv1.Variable{},
	}, {
		name: "not empty",
		policy: &ImageVerificationPolicy{
			Spec: ImageVerificationPolicySpec{
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

func TestImageVerificationPolicy_GetSpec(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageVerificationPolicy
		want   *ImageVerificationPolicySpec
	}{{
		name: "empty",
		policy: &ImageVerificationPolicy{
			Spec: ImageVerificationPolicySpec{
				Variables: []admissionregistrationv1.Variable{},
			},
		},
		want: &ImageVerificationPolicySpec{
			Variables: []admissionregistrationv1.Variable{},
		},
	}, {
		name: "not empty",
		policy: &ImageVerificationPolicy{
			Spec: ImageVerificationPolicySpec{
				Variables: []admissionregistrationv1.Variable{{
					Name:       "dummy",
					Expression: "expression",
				}},
			},
		},
		want: &ImageVerificationPolicySpec{
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

func TestImageVerificationPolicy_GetStatus(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageVerificationPolicy
		want   *ConditionStatus
	}{{
		policy: &ImageVerificationPolicy{},
		want:   &ConditionStatus{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.GetStatus()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestImageVerificationPolicy_GetKind(t *testing.T) {
	tests := []struct {
		name   string
		policy *ImageVerificationPolicy
		want   string
	}{{
		name:   "not set",
		policy: &ImageVerificationPolicy{},
		want:   "ImageVerificationPolicy",
	}, {
		name: "set",
		policy: &ImageVerificationPolicy{
			TypeMeta: v1.TypeMeta{
				Kind: "Foo",
			},
		},
		want: "ImageVerificationPolicy",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.GetKind()
			assert.Equal(t, tt.want, got)
		})
	}
}
