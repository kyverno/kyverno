package policyexceptions

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

// MockPolicyException is a basic implementation of the policy exception interface.
// It is temporarily used by the resourceHandler to pass the compilation.
// It is supposed to be used for testing in the future.
type MockPolicyException struct{}

func (m MockPolicyException) ExceptionsByRule(policy kyvernov1.PolicyInterface, ruleName string) ExcludeResource {
	result := make([]kyvernov1.MatchResources, 0)
	return result
}

func (m MockPolicyException) IsNil() bool {
	return false
}

func NewMockPolicyException() MockPolicyException {
	return MockPolicyException{}
}
