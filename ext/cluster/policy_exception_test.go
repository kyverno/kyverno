package cluster

import (
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPolicyExceptionSelector_Find_FromAdditional(t *testing.T) {
	// GIVEN: a policy exception
	exception := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-exception",
			Namespace: "default",
		},
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{
				{
					PolicyName: "test-policy",
					RuleNames:  []string{"test-rule"},
				},
			},
		},
	}

	// AND: selector with no client, only additional exceptions
	var selector engineapi.PolicyExceptionSelector = NewPolicyExceptionSelector(
		"default",
		nil,
		exception,
	)

	// WHEN: searching for matching policy + rule
	result, err := selector.Find("test-policy", "test-rule")

	// THEN
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "test-exception", result[0].Name)
}

func TestPolicyExceptionSelector_Find_NamespaceMismatch(t *testing.T) {
	exception := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ns-mismatch",
			Namespace: "kube-system",
		},
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{
				{
					PolicyName: "policy",
					RuleNames:  []string{"rule"},
				},
			},
		},
	}

	selector := NewPolicyExceptionSelector(
		"default",
		nil,
		exception,
	)

	result, err := selector.Find("policy", "rule")

	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestPolicyExceptionSelector_Find_RuleMismatch(t *testing.T) {
	exception := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rule-mismatch",
			Namespace: "default",
		},
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{
				{
					PolicyName: "policy",
					RuleNames:  []string{"other"},
				},
			},
		},
	}

	selector := NewPolicyExceptionSelector(
		"default",
		nil,
		exception,
	)

	result, err := selector.Find("policy", "rule")

	assert.NoError(t, err)
	assert.Len(t, result, 1)
}
