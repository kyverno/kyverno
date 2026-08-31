package matching

import (
	"slices"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

// ExpandPodSubresources returns a copy of the match constraints where core/v1
// Pod rules also match the ephemeralcontainers subresource. This keeps CEL
// policy matching aligned with the webhook rules generated for legacy Kyverno
// policies.
func ExpandPodSubresources(match *admissionregistrationv1.MatchResources) *admissionregistrationv1.MatchResources {
	if match == nil {
		return nil
	}

	expanded := match.DeepCopy()
	expandPodSubresources(expanded.ResourceRules)
	expandPodSubresources(expanded.ExcludeResourceRules)
	return expanded
}

func expandPodSubresources(rules []admissionregistrationv1.NamedRuleWithOperations) {
	for i := range rules {
		rule := &rules[i].RuleWithOperations.Rule
		if matches(rule.APIGroups, "") &&
			matches(rule.APIVersions, "v1") &&
			matches(rule.Resources, "pods") &&
			!slices.Contains(rule.Resources, "pods/ephemeralcontainers") {
			rule.Resources = append(rule.Resources, "pods/ephemeralcontainers")
		}
	}
}

func matches(values []string, value string) bool {
	return slices.Contains(values, value) || slices.Contains(values, "*")
}
