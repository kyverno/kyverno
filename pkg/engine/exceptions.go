package engine

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"k8s.io/client-go/tools/cache"
)

// GetPolicyExceptions get all exceptions that match both the policy and the rule.
func (e *engine) GetPolicyExceptions(
	policy kyvernov1.PolicyInterface,
	rule string,
) ([]*kyvernov2.PolicyException, error) {
	if e.exceptionSelector == nil {
		return nil, nil
	}
	return e.exceptionSelector.Find(cache.MetaObjectToName(policy).String(), rule)
}
