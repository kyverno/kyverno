package engine

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"k8s.io/client-go/tools/cache"
)

// GetPolicyExceptions get all exceptions that match both the policy and the rule.
func (e *engine) GetPolicyExceptions(
	policy kyvernov1.PolicyInterface,
	ruleName string,
) ([]*kyvernov2beta1.PolicyException, error) {
	if e.exceptionSelector == nil {
		return nil, nil
	}

	policyName := cache.MetaObjectToName(policy).String()
	exceptions, err := e.exceptionSelector.GetPolicyExceptionsByPolicyRulePair(policyName, ruleName)
	if err != nil {
		return nil, err
	}

	return exceptions, nil
}
