package processor

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

func policyHasGenerate(policy kyvernov1.PolicyInterface) bool {
	for _, rule := range policy.GetSpec().Rules {
		if rule.HasGenerate() {
			return true
		}
	}
	return false
}
