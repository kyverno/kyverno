package processor

import (
	"context"

	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiserver/pkg/admission"
)

// triggeredMutateExistingPolicies returns the keys of the mutate-existing policies whose trigger rules,
// matchConstraints and matchConditions, match the resource described by attr. Like the admission
// matcher, a namespaced policy only applies to resources in its own namespace. It is the check the
// admission webhook makes through MatchedMutateExistingPolicies, without that method's admission-enabled
// gate: the CLI also runs background-only policies against the target resources it is given, the way
// the background controller does.
func triggeredMutateExistingPolicies(
	ctx context.Context,
	provider mpolengine.Provider,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace *corev1.Namespace,
	contextProvider libs.Context,
) []string {
	var keys []string
	matcher := matching.NewMatcher()
	inScope := mpolengine.Or(mpolengine.ClusteredPolicy(), mpolengine.NamespacedPolicy(attr.GetNamespace()))
	for _, mpol := range provider.Fetch(ctx, true) {
		if !inScope(mpol.Policy) {
			continue
		}
		spec := mpol.Policy.GetSpec()
		if ok, err := matcher.Match(&matching.MatchCriteria{Constraints: spec.MatchConstraints}, attr, namespace); err != nil || !ok {
			continue
		}
		if spec.MatchConditions != nil && !mpol.CompiledPolicy.MatchesConditions(ctx, attr, request, namespace, nil, contextProvider) {
			continue
		}
		keys = append(keys, mpolengine.PolicyKey(mpol.Policy))
	}
	return keys
}
