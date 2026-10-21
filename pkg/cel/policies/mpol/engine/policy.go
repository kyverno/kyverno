package engine

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
)

type Policy struct {
	Policy         policiesv1beta1.MutatingPolicyLike
	CompiledPolicy *compiler.Policy
	// ExtractionMode is true for an autogen'd target whose pod template is
	// discovered by structural extraction at evaluation time (custom
	// workload CRDs like JobSet) rather than by matching the literal
	// admitted object directly. Mutation is not supported for these targets
	// yet - see engineImpl.handlePolicy.
	ExtractionMode bool
}
