package engine

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/polex/store"
	"k8s.io/client-go/tools/cache"
)

// GetPolicyExceptions get all exceptions that match both the policy and the rule.
func (e *engine) GetPolicyExceptions(
	policy kyvernov1.PolicyInterface,
	rule string,
) ([]*kyvernov2beta1.PolicyException, error) {
	var exceptions []*kyvernov2beta1.PolicyException
	if e.exceptionStore == nil {
		return exceptions, nil
	}

	policyName, err := cache.MetaNamespaceKeyFunc(policy)
	if err != nil {
		return exceptions, fmt.Errorf("failed to compute policy key: %w", err)
	}

	storeExceptions, ok := e.exceptionStore.Get(store.Key{
		PolicyName: policyName,
		RuleName:   rule,
	})
	if ok {
		return storeExceptions, nil
	}

	return exceptions, nil
}
