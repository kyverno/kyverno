package compiler

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type EvaluationResult struct {
	Error      error
	Message    string
	Result     bool
	Exceptions []*policiesv1alpha1.PolicyException
}

type evaluationData struct {
	Object    any
	Context   libs.Context
	Variables *lazy.MapValue
}
