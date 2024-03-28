package api

import (
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
)

// PolicyExceptionSelector is an abstract interface used to resolve poliicy exceptions
type PolicyExceptionSelector interface {
	// Find returns policy exceptions matching a given policy name and rule name.
	// Objects returned here must be treated as read-only.
	Find(string, string) ([]*kyvernov2beta1.PolicyException, error)
}
