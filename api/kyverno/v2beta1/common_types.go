package v2beta1

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

// Deny specifies a list of conditions used to pass or fail a validation rule.
type Deny struct {
	// Multiple conditions can be declared under an `any` or `all` statement. A direct list
	// of conditions (without `any` or `all` statements) is also supported for backwards compatibility
	// but will be deprecated in the next major release.
	// See: https://kyverno.io/docs/writing-policies/validate/#deny-rules
	RawAnyAllConditions *kyvernov1.AnyAllConditions `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}
