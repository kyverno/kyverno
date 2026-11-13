package policies

import (
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestRemoveNoneBackgroundPolicies(t *testing.T) {
	yes := v1beta1.ValidatingPolicy{
		Spec: v1beta1.ValidatingPolicySpec{
			EvaluationConfiguration: &v1beta1.EvaluationConfiguration{
				Background: &v1beta1.BackgroundConfiguration{
					Enabled: ptr.To(true),
				},
			},
		},
	}
	no := v1beta1.ValidatingPolicy{
		Spec: v1beta1.ValidatingPolicySpec{
			EvaluationConfiguration: &v1beta1.EvaluationConfiguration{
				Background: &v1beta1.BackgroundConfiguration{
					Enabled: ptr.To(false),
				},
			},
		},
	}
	tests := []struct {
		name     string
		policies []v1beta1.ValidatingPolicy
		want     []v1beta1.ValidatingPolicy
	}{{
		name:     "nil",
		policies: nil,
		want:     []v1beta1.ValidatingPolicy{},
	}, {
		name:     "empty",
		policies: []v1beta1.ValidatingPolicy{},
		want:     []v1beta1.ValidatingPolicy{},
	}, {
		name:     "only no",
		policies: []v1beta1.ValidatingPolicy{no},
		want:     []v1beta1.ValidatingPolicy{},
	}, {
		name:     "only yes",
		policies: []v1beta1.ValidatingPolicy{yes},
		want:     []v1beta1.ValidatingPolicy{yes},
	}, {
		name:     "both",
		policies: []v1beta1.ValidatingPolicy{yes, no},
		want:     []v1beta1.ValidatingPolicy{yes},
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RemoveNoneBackgroundPolicies(tt.policies)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsKyvernoEvaluationMode(t *testing.T) {
	assert.True(t, IsKyvernoEvaluationMode(""))
	assert.True(t, IsKyvernoEvaluationMode(policieskyvernoio.EvaluationModeKubernetes))
	assert.True(t, IsKyvernoEvaluationMode(policieskyvernoio.EvaluationModeJSON))
	assert.False(t, IsKyvernoEvaluationMode("Envoy"))
	assert.False(t, IsKyvernoEvaluationMode("HTTP"))
	assert.False(t, IsKyvernoEvaluationMode("ext_proc"))
}

func TestSelectKyvernoExceptions(t *testing.T) {
	owned := &v1beta1.PolicyException{}
	envoy := &v1beta1.PolicyException{Spec: v1beta1.PolicyExceptionSpec{EvaluationMode: "Envoy"}}
	selected := SelectKyvernoExceptions([]*v1beta1.PolicyException{owned, nil, envoy})
	assert.Equal(t, []*v1beta1.PolicyException{owned}, selected)
	assert.Nil(t, SelectKyvernoExceptions(nil))
	assert.Empty(t, SelectKyvernoExceptions([]*v1beta1.PolicyException{}))

	errs := ForeignEvaluationModeError("Envoy")
	if assert.Len(t, errs, 1) {
		assert.Equal(t, "Envoy", errs[0].BadValue)
	}
}
