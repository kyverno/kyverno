package engine

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"k8s.io/client-go/tools/cache"
)

// GetPolicyExceptions get all exceptions that match both the policy and the rule.
func (e *engine) GetPolicyExceptions(
	policy kyvernov1.PolicyInterface,
	rule string,
) ([]kyvernov2beta1.PolicyException, error) {
	var exceptions []kyvernov2beta1.PolicyException
	if e.exceptionSelector == nil {
		return exceptions, nil
	}
	policyName := cache.MetaObjectToName(policy).String()
	polexs, err := e.exceptionSelector.Find(policyName, rule)
	if err != nil {
		return exceptions, err
	}
	for _, polex := range polexs {
		exceptions = append(exceptions, *polex)
	}
	return exceptions, nil
}
