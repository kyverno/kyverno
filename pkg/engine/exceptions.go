package engine

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"k8s.io/client-go/tools/cache"
)

// GetPolicyExceptions get all exceptions that match both the policy and the rule.
func (e *engine) GetPolicyExceptions(
	policy kyvernov1.PolicyInterface,
) ([]*kyvernov2beta1.PolicyException, error) {
	if e.exceptionSelector == nil {
		return nil, nil
	}
	policyName := cache.MetaObjectToName(policy).String()
	return e.exceptionSelector.Find(policyName)
}
