package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
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
