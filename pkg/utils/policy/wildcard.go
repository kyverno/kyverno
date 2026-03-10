package policy

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/ext/wildcard"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

// WildcardWarning is the standard warning message for wildcard policies.
const WildcardWarning = "wildcard policy adds high load to the API server and can bring down a cluster"

// HasWildcard returns true if the policy contains a wildcard '*' in its
// match or exclude constraints. It covers standard Kyverno policies,
// CEL-based policies, and native Kubernetes admission policies.
func HasWildcard(policy engineapi.GenericPolicy) bool {
	if policy == nil {
		return false
	}
	// Standard Kyverno policy
	if kp := policy.AsKyvernoPolicy(); kp != nil {
		for _, rule := range autogen.Default.ComputeRules(kp, "") {
			if hasWildcardInKyvernoMatch(rule.MatchResources) {
				return true
			}
			if rule.ExcludeResources != nil && hasWildcardInKyvernoMatch(*rule.ExcludeResources) {
				return true
			}
		}
	}
	// Native ValidatingAdmissionPolicy
	if vap := policy.AsValidatingAdmissionPolicy(); vap != nil {
		if def := vap.GetDefinition(); def != nil && hasWildcardInNativeRules(def.Spec.MatchConstraints) {
			return true
		}
	}
	// Native MutatingAdmissionPolicy (v1beta1 — same struct shape)
	if mapData := policy.AsMutatingAdmissionPolicy(); mapData != nil {
		if def := mapData.GetDefinition(); def != nil && def.Spec.MatchConstraints != nil {
			for _, r := range def.Spec.MatchConstraints.ResourceRules {
				if hasWildcardInGVR(r.Rule.APIGroups, r.Rule.APIVersions, r.Rule.Resources) {
					return true
				}
			}
			for _, r := range def.Spec.MatchConstraints.ExcludeResourceRules {
				if hasWildcardInGVR(r.Rule.APIGroups, r.Rule.APIVersions, r.Rule.Resources) {
					return true
				}
			}
		}
	}
	// CEL-based policies (all use admissionregistrationv1.MatchResources)
	if vpol := policy.AsValidatingPolicyLike(); vpol != nil {
		mc := vpol.GetMatchConstraints()
		if hasWildcardInNativeRules(&mc) {
			return true
		}
	}
	if mpol := policy.AsMutatingPolicyLike(); mpol != nil {
		mc := mpol.GetMatchConstraints()
		if hasWildcardInNativeRules(&mc) {
			return true
		}
	}
	if gpol := policy.AsGeneratingPolicyLike(); gpol != nil {
		mc := gpol.GetMatchConstraints()
		if hasWildcardInNativeRules(&mc) {
			return true
		}
	}
	if ivpol := policy.AsImageValidatingPolicyLike(); ivpol != nil {
		mc := ivpol.GetMatchConstraints()
		if hasWildcardInNativeRules(&mc) {
			return true
		}
	}
	return false
}

// hasWildcardInKyvernoMatch checks Kyverno MatchResources for wildcards in Kinds.
func hasWildcardInKyvernoMatch(match kyvernov1.MatchResources) bool {
	for _, kind := range match.ResourceDescription.Kinds {
		if wildcard.ContainsWildcard(kind) {
			return true
		}
	}
	for _, rf := range match.Any {
		for _, kind := range rf.ResourceDescription.Kinds {
			if wildcard.ContainsWildcard(kind) {
				return true
			}
		}
	}
	for _, rf := range match.All {
		for _, kind := range rf.ResourceDescription.Kinds {
			if wildcard.ContainsWildcard(kind) {
				return true
			}
		}
	}
	return false
}

// hasWildcardInNativeRules checks Kubernetes-native MatchResources for wildcards.
func hasWildcardInNativeRules(match *admissionregistrationv1.MatchResources) bool {
	if match == nil {
		return false
	}
	for _, r := range match.ResourceRules {
		if hasWildcardInGVR(r.Rule.APIGroups, r.Rule.APIVersions, r.Rule.Resources) {
			return true
		}
	}
	for _, r := range match.ExcludeResourceRules {
		if hasWildcardInGVR(r.Rule.APIGroups, r.Rule.APIVersions, r.Rule.Resources) {
			return true
		}
	}
	return false
}

// hasWildcardInGVR checks APIGroups, APIVersions, and Resources for wildcards.
func hasWildcardInGVR(groups, versions, resources []string) bool {
	for _, g := range groups {
		if g == "*" {
			return true
		}
	}
	for _, v := range versions {
		if v == "*" {
			return true
		}
	}
	for _, r := range resources {
		if wildcard.ContainsWildcard(r) {
			return true
		}
	}
	return false
}
