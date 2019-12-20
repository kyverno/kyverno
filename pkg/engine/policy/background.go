package policy

import (
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/variables"
)

//ContainsUserInfo returns true if contains userInfo
func ContainsUserInfo(policy kyverno.ClusterPolicy) bool {
	// iterate of the policy rules to identify if userInfo is used
	for _, rule := range policy.Spec.Rules {
		if len(rule.MatchResources.ClusterRoles) > 0 {
			// user-role has been defined
			return true
		}
		if len(rule.ExcludeResources.ClusterRoles) > 0 {
			// user-role has been defined
			return true
		}

		// variable defined with user information
		// - mutate.overlay
		// - validate.pattern
		// - validate.anyPattern[*]
		// variables to filter
		// - request.userInfo
		filterVars := []string{"request.userInfo*"}
		if exists := variables.CheckVariables(rule.Mutation.Overlay, filterVars); exists {
			return false
		}
		if exists := variables.CheckVariables(rule.Validation.Pattern, filterVars); exists {
			return false
		}
		for _, pattern := range rule.Validation.AnyPattern {
			if exists := variables.CheckVariables(pattern, filterVars); exists {
				return false
			}
		}
	}
	return false
}
