package compiler

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/trace"
)

type EvaluationResult struct {
	Error      error
	Message    string
	Result     bool
	Exceptions []*policiesv1beta1.PolicyException
	// Trace is the decision trace for this evaluation. It is nil unless the policy was compiled
	// with tracing on, so callers must nil-check it. Only Match (the delete conditions),
	// Variables and Verdict are filled here; the policy/resource header and Scope are left for
	// the engine to fill in. When Evaluate fails it can be returned alongside the error, carrying
	// what was traced before the failure.
	Trace *trace.Decision
}
