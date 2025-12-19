package compiler

import (
	policiesv1beta1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1beta1"
)

type EvaluationResult struct {
	Error      error
	Message    string
	Result     bool
	Exceptions []*policiesv1beta1.PolicyException
}
