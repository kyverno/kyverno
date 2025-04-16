package autogen

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

// CanAutoGen checks whether the policy can be applied to Pod controllers
// It returns false if:
//   - the matching logic has a namespace selector
//   - the matching logic has an object selector
//   - the matching logic has exclusion rules
//   - the matching logic matches more than one resource and it's not pods
//   - the matching logic filters on resource names
func CanAutoGen(match *admissionregistrationv1.MatchResources) bool {
	if match == nil {
		return false
	}
	if match.NamespaceSelector != nil {
		if len(match.NamespaceSelector.MatchLabels) > 0 || len(match.NamespaceSelector.MatchExpressions) > 0 {
			return false
		}
	}
	if match.ObjectSelector != nil {
		if len(match.ObjectSelector.MatchLabels) > 0 || len(match.ObjectSelector.MatchExpressions) > 0 {
			return false
		}
	}
	if len(match.ExcludeResourceRules) != 0 {
		return false
	}
	if len(match.ResourceRules) != 1 {
		return false
	}
	rule := match.ResourceRules[0]
	if len(rule.ResourceNames) > 0 {
		return false
	}
	if len(rule.APIGroups) != 1 || rule.APIGroups[0] != "" {
		return false
	}
	if len(rule.APIVersions) != 1 || rule.APIVersions[0] != "v1" {
		return false
	}
	if len(rule.Resources) != 1 || rule.Resources[0] != "pods" {
		return false
	}
	return true
}
