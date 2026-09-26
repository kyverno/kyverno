package matching

import (
	"fmt"
	"strings"

	"github.com/kyverno/kyverno/pkg/cel/matching/predicates/namespace"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/predicates/object"
)

// Explain describes in words why a policy's match constraints did or did not apply to a request.
//
// It never decides the outcome: Match is the authority and its result is passed in as matched.
// When it did not match, Explain re-checks the same stages Match runs, in the same order, to name
// the first one that rejected the request. If none of them does (which would mean the two have
// drifted apart) it says so generically rather than guess.
func Explain(constraints *admissionregistrationv1.MatchResources, attr admission.Attributes, ns runtime.Object, matched bool) string {
	if constraints == nil {
		return "the policy has no matchConstraints, so it applies to nothing"
	}
	if matched {
		return "matched " + describeRequest(attr)
	}

	criteria := &MatchCriteria{Constraints: constraints}
	nsMatcher := namespace.Matcher{Namespace: ns}
	if ok, err := nsMatcher.MatchNamespaceSelector(criteria, attr); err == nil && !ok {
		return fmt.Sprintf("namespace %q does not satisfy the policy's namespaceSelector", attr.GetNamespace())
	}
	if ok, err := (&object.Matcher{}).MatchObjectSelector(criteria, attr); err == nil && !ok {
		return "the object's labels do not satisfy the policy's objectSelector"
	}
	if excluded, err := matchesResourceRules(constraints.ExcludeResourceRules, attr); err == nil && excluded {
		return fmt.Sprintf("%s is excluded by the policy's excludeResourceRules", describeRequest(attr))
	}
	if len(constraints.ResourceRules) > 0 {
		if ok, err := matchesResourceRules(constraints.ResourceRules, attr); err == nil && !ok {
			return fmt.Sprintf("%s is not covered by the policy's resourceRules (%s)", describeRequest(attr), describeRules(constraints.ResourceRules))
		}
	}
	return fmt.Sprintf("%s did not match the policy's matchConstraints", describeRequest(attr))
}

func describeRequest(attr admission.Attributes) string {
	parts := []string{"kind " + attr.GetKind().Kind}
	if ns := attr.GetNamespace(); ns != "" {
		parts = append(parts, "namespace "+ns)
	} else {
		parts = append(parts, "cluster-scoped")
	}
	parts = append(parts, "operation "+string(attr.GetOperation()))
	return strings.Join(parts, ", ")
}

func describeRules(rules []admissionregistrationv1.NamedRuleWithOperations) string {
	described := make([]string, 0, len(rules))
	for _, r := range rules {
		described = append(described, fmt.Sprintf("apiGroups=%q resources=%q operations=%v", r.APIGroups, r.Resources, r.Operations))
	}
	return strings.Join(described, "; ")
}
