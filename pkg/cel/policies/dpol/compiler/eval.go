package compiler

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
)

type EvaluationResult struct {
	Error      error
	Message    string
	Result     bool
	Exceptions []*policiesv1alpha1.PolicyException
}
