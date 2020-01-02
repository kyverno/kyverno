package policy

import (
	"fmt"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/variables"
)

//ContainsUserInfo returns error is userInfo is defined
func ContainsUserInfo(policy kyverno.ClusterPolicy) error {
	// iterate of the policy rules to identify if userInfo is used
	for idx, rule := range policy.Spec.Rules {
		if err := userInfoDefined(rule.MatchResources.UserInfo); err != nil {
			return fmt.Errorf("path: spec/rules[%d]/match/%s", idx, err)
		}

		if err := userInfoDefined(rule.ExcludeResources.UserInfo); err != nil {
			return fmt.Errorf("path: spec/rules[%d]/exclude/%s", idx, err)
		}

		// variable defined with user information
		// - mutate.overlay
		// - validate.pattern
		// - validate.anyPattern[*]
		// variables to filter
		// - request.userInfo
		filterVars := []string{"request.userInfo*"}
		if err := variables.CheckVariables(rule.Mutation.Overlay, filterVars, "/"); err != nil {
			return fmt.Errorf("path: spec/rules[%d]/mutate/overlay%s", idx, err)
		}
		if err := variables.CheckVariables(rule.Validation.Pattern, filterVars, "/"); err != nil {
			return fmt.Errorf("path: spec/rules[%d]/validate/pattern%s", idx, err)
		}
		for idx2, pattern := range rule.Validation.AnyPattern {
			if err := variables.CheckVariables(pattern, filterVars, "/"); err != nil {
				return fmt.Errorf("path: spec/rules[%d]/validate/anyPattern[%d]%s", idx, idx2, err)
			}
		}
	}
	return nil
}

func userInfoDefined(ui kyverno.UserInfo) error {
	if len(ui.Roles) > 0 {
		return fmt.Errorf("roles")
	}
	if len(ui.ClusterRoles) > 0 {
		return fmt.Errorf("clusterRoles")
	}
	if len(ui.Subjects) > 0 {
		return fmt.Errorf("subjects")
	}
	return nil
}
