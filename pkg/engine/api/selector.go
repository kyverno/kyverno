package api

import (
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
)

// PolicyExceptionSelector is an abstract interface used to resolve poliicy exceptions
type PolicyExceptionSelector interface {
	GetPolicyExceptionsByPolicyRulePair(policyName, ruleName string) ([]*kyvernov2beta1.PolicyException, error)
}
