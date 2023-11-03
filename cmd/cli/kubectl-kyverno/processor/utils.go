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

func policyHasMutate(policy kyvernov1.PolicyInterface) bool {
	for _, rule := range policy.GetSpec().Rules {
		if rule.HasMutate() {
			return true
		}
	}
	return false
}

func policyHasValidateOrVerifyImageChecks(policy kyvernov1.PolicyInterface) bool {
	for _, rule := range policy.GetSpec().Rules {
		//  engine.validate handles both validate and verifyImageChecks atm
		if rule.HasValidate() || rule.HasVerifyImageChecks() {
			return true
		}
	}
	return false
}

func policyHasVerifyImages(policy kyvernov1.PolicyInterface) bool {
	for _, rule := range policy.GetSpec().Rules {
		if rule.HasVerifyImages() {
			return true
		}
	}
	return false
}
