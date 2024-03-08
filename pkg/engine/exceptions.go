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
) ([]*kyvernov2beta1.PolicyException, error) {
	if e.exceptionSelector == nil {
		return nil, nil
	}
	policyName := cache.MetaObjectToName(policy).String()
	polexs, err := e.exceptionSelector.Find(policyName)
	if err != nil {
		return nil, err
	}
	exceptions := make([]*kyvernov2beta1.PolicyException, 0, len(polexs))
	for _, polex := range polexs {
		if polex.Contains(policyName, rule) {
			exceptions = append(exceptions, polex)
		}
	}
	return exceptions, nil
}
