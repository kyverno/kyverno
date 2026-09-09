package engine

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type Policy struct {
	Actions        sets.Set[admissionregistrationv1.ValidationAction]
	Policy         policiesv1beta1.ValidatingPolicyLike
	CompiledPolicy *compiler.Policy
	// ExtractionMode is true for an autogen'd target whose pod template is
	// discovered by structural extraction at evaluation time (custom
	// workload CRDs like JobSet) rather than by matching the literal
	// admitted object directly against CompiledPolicy.
	ExtractionMode bool
}
