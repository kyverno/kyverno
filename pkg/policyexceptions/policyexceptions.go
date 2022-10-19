package policyexceptions

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

type ExcludeResource []kyvernov1.MatchResources

// We need to create objects for this interface so that
// resourceHandler can easily call the method to get policy exceptions
type Interface interface {
	// Get corresponding exceptions by policy and rule
	// Return `ExcludeResource` which is an array, including elements:
	// `kyvernov1.MatchResources`
	ExceptionsByRule(policy kyvernov1.PolicyInterface, ruleName string) ExcludeResource

	// Check if the object is nil
	// Return bool
	IsNil() bool
}
