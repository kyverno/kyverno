package policies

import (
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
)

func TestIsJSONMutatingPolicy(t *testing.T) {
	t.Parallel()
	var nilPolicy *policiesv1beta1.MutatingPolicy
	tests := []struct {
		name   string
		policy policiesv1beta1.MutatingPolicyLike
		want   bool
	}{
		{"nil interface", nil, false},
		{"nil pointer", nilPolicy, false},
		{"default mode", &policiesv1beta1.MutatingPolicy{}, false},
		{"explicit kubernetes", &policiesv1beta1.MutatingPolicy{Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeKubernetes},
		}}, false},
		{"json cluster policy", &policiesv1beta1.MutatingPolicy{Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
		}}, true},
		{"json namespaced policy", &policiesv1beta1.NamespacedMutatingPolicy{Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
		}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, IsJSONMutatingPolicy(tt.policy))
		})
	}
}
