package engine

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"k8s.io/apimachinery/pkg/labels"
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
	polexs, err := e.exceptionSelector.List(labels.Everything())
	if err != nil {
		return exceptions, err
	}
	policyName, err := cache.MetaNamespaceKeyFunc(policy)
	if err != nil {
		return exceptions, fmt.Errorf("failed to compute policy key: %w", err)
	}
	for _, polex := range polexs {
		if polex.Contains(policyName, rule) {
			exceptions = append(exceptions, *polex)
		}
	}
	return exceptions, nil
}
