package policies

import (
	"testing"

	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestRemoveNoneBackgroundPolicies(t *testing.T) {
	yes := v1alpha1.ValidatingPolicy{
		Spec: v1alpha1.ValidatingPolicySpec{
			EvaluationConfiguration: &v1alpha1.EvaluationConfiguration{
				Background: &v1alpha1.BackgroundConfiguration{
					Enabled: ptr.To(true),
				},
			},
		},
	}
	no := v1alpha1.ValidatingPolicy{
		Spec: v1alpha1.ValidatingPolicySpec{
			EvaluationConfiguration: &v1alpha1.EvaluationConfiguration{
				Background: &v1alpha1.BackgroundConfiguration{
					Enabled: ptr.To(false),
				},
			},
		},
	}
	tests := []struct {
		name     string
		policies []v1alpha1.ValidatingPolicy
		want     []v1alpha1.ValidatingPolicy
	}{{
		name:     "nil",
		policies: nil,
		want:     []v1alpha1.ValidatingPolicy{},
	}, {
		name:     "empty",
		policies: []v1alpha1.ValidatingPolicy{},
		want:     []v1alpha1.ValidatingPolicy{},
	}, {
		name:     "only no",
		policies: []v1alpha1.ValidatingPolicy{no},
		want:     []v1alpha1.ValidatingPolicy{},
	}, {
		name:     "only yes",
		policies: []v1alpha1.ValidatingPolicy{yes},
		want:     []v1alpha1.ValidatingPolicy{yes},
	}, {
		name:     "both",
		policies: []v1alpha1.ValidatingPolicy{yes, no},
		want:     []v1alpha1.ValidatingPolicy{yes},
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RemoveNoneBackgroundPolicies(tt.policies)
			assert.Equal(t, tt.want, got)
		})
	}
}
