package processor

import (
	"context"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
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

// policyKeyIn matches the policies whose mpolengine.PolicyKey is one of keys. Unlike MatchNames it never
// matches a namespaced policy by its bare name, so the key of a clustered policy cannot select a
// namespaced policy with the same name, and no keys match no policy.
func policyKeyIn(keys []string) mpolengine.Predicate {
	set := sets.New(keys...)
	return func(policy policiesv1beta1.MutatingPolicyLike) bool {
		return set.Has(mpolengine.PolicyKey(policy))
	}
}

// celTargetKey identifies a target selected by a CEL targetMatchConstraints expression by its group,
// kind, namespace and name, so a selected object never matches another kind with the same name.
func celTargetKey(gk schema.GroupKind, namespace, name string) string {
	key := name
	if namespace != "" {
		key = namespace + "/" + name
	}
	return gk.String() + "|" + key
}

// admittedTrigger is a snapshot of the trigger as it was admitted, taken before inline mutations change the
// resource the processor works on.
type admittedTrigger struct {
	operation admission.Operation
	oldObject runtime.Object
	request   celengine.EngineRequest
	attr      admission.Attributes
}

// newAdmittedTrigger builds the admission request and attributes of trigger, which must be a copy the
// processor does not mutate afterwards.
func newAdmittedTrigger(
	contextProvider libs.Context,
	trigger *unstructured.Unstructured,
	gvk schema.GroupVersionKind,
	gvr schema.GroupVersionResource,
	subresource string,
	operation string,
	user authenticationv1.UserInfo,
) admittedTrigger {
	op, object, oldObject := AdmissionRequestShape(operation, trigger)
	return admittedTrigger{
		operation: admission.Operation(op),
		oldObject: oldObject,
		request:   celengine.Request(contextProvider, gvk, gvr, subresource, trigger.GetName(), trigger.GetNamespace(), op, user, object, oldObject, false, nil),
		attr:      admission.NewAttributesRecord(object, oldObject, gvk, trigger.GetNamespace(), trigger.GetName(), gvr, subresource, admission.Operation(op), nil, false, admissionpolicy.NewUser(user)),
	}
}

// conditionsStillMatch keeps the triggered policies whose matchConditions still hold for the trigger as it
// is stored after inline mutation, described by attr. The background controller checks them again this way
// before it mutates any target, so a policy triggered at admission can still drop out here.
func conditionsStillMatch(
	ctx context.Context,
	provider mpolengine.Provider,
	triggered []string,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace *corev1.Namespace,
	contextProvider libs.Context,
) []string {
	keys := sets.New(triggered...)
	var kept []string
	for _, mpol := range provider.Fetch(ctx, true) {
		key := mpolengine.PolicyKey(mpol.Policy)
		if !keys.Has(key) {
			continue
		}
		if mpol.Policy.GetSpec().MatchConditions != nil && !mpol.CompiledPolicy.MatchesConditions(ctx, attr, request, namespace, nil, contextProvider) {
			continue
		}
		kept = append(kept, key)
	}
	return kept
}
