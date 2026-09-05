package engine

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type Policy struct {
	Policy     policiesv1beta1.ImageValidatingPolicyLike
	Exceptions []*policiesv1beta1.PolicyException
	Actions    sets.Set[admissionregistrationv1.ValidationAction]
	// ExtractionMode is true for an autogen'd target whose pod template is
	// discovered by structural extraction at evaluation time (custom
	// workload CRDs like JobSet) rather than by matching the literal
	// admitted object directly. Digest-pinning mutation is not supported
	// for these targets yet - see HandleMutating.
	ExtractionMode bool
}
