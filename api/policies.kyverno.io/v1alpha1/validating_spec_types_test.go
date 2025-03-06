package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestValidatingPolicySpec_AdmissionEnabled(t *testing.T) {
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
					Admission: &AdmissionConfiguration{
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

func TestValidatingPolicySpec_BackgroundEnabled(t *testing.T) {
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
			got := tt.policy.Spec.BackgroundEnabled()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidatingPolicySpec_EvaluationMode(t *testing.T) {
	tests := []struct {
		name   string
		policy *ValidatingPolicy
		want   EvaluationMode
	}{{
		name:   "nil",
		policy: &ValidatingPolicy{},
		want:   EvaluationModeKubernetes,
	}, {
		name: "json",
		policy: &ValidatingPolicy{
			Spec: ValidatingPolicySpec{
				EvaluationConfiguration: &EvaluationConfiguration{
					Mode: EvaluationModeJSON,
				},
			},
		},
		want: EvaluationModeJSON,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.Spec.EvaluationMode()
			assert.Equal(t, tt.want, got)
		})
	}
}
