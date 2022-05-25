package policy

import (
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
)

// ContainsUserVariables returns error if variable that does not start from request.object
func containsUserVariables(policy kyvernov1.PolicyInterface, vars [][]string) error {
	for _, rule := range policy.GetSpec().Rules {
		if rule.IsMutateExisting() {
			return nil
		}
	}

	for _, s := range vars {
		if strings.Contains(s[0], "userInfo") {
			return fmt.Errorf("variable %s is not allowed", s[0])
		}
	}
	rules := autogen.ComputeRules(policy)
	for idx := range rules {
		if err := hasUserMatchExclude(idx, &rules[idx]); err != nil {
			return err
		}
	}

	return nil
}

func hasUserMatchExclude(idx int, rule *kyvernov1.Rule) error {
	if path := userInfoDefined(rule.MatchResources.UserInfo); path != "" {
		return fmt.Errorf("invalid variable used at path: spec/rules[%d]/match/%s", idx, path)
	}

	if path := userInfoDefined(rule.ExcludeResources.UserInfo); path != "" {
		return fmt.Errorf("invalid variable used at path: spec/rules[%d]/exclude/%s", idx, path)
	}

	if len(rule.MatchResources.Any) > 0 {
		for i, value := range rule.MatchResources.Any {
			if path := userInfoDefined(value.UserInfo); path != "" {
				return fmt.Errorf("invalid variable used at path: spec/rules[%d]/match/any[%d]/%s", idx, i, path)
			}
		}
	}

	if len(rule.MatchResources.All) > 0 {
		for i, value := range rule.MatchResources.All {
			if path := userInfoDefined(value.UserInfo); path != "" {
				return fmt.Errorf("invalid variable used at path: spec/rules[%d]/match/all[%d]/%s", idx, i, path)
			}
		}
	}

	if len(rule.ExcludeResources.All) > 0 {
		for i, value := range rule.ExcludeResources.All {
			if path := userInfoDefined(value.UserInfo); path != "" {
				return fmt.Errorf("invalid variable used at path: spec/rules[%d]/exclude/any[%d]/%s", idx, i, path)
			}
		}
	}

	if len(rule.ExcludeResources.Any) > 0 {
		for i, value := range rule.ExcludeResources.Any {
			if path := userInfoDefined(value.UserInfo); path != "" {
				return fmt.Errorf("invalid variable used at path: spec/rules[%d]/exclude/all[%d]/%s", idx, i, path)
			}
		}
	}

	return nil
}

func userInfoDefined(ui kyvernov1.UserInfo) string {
	if len(ui.Roles) > 0 {
		return "roles"
	}
	if len(ui.ClusterRoles) > 0 {
		return "clusterRoles"
	}
	if len(ui.Subjects) > 0 {
		return "subjects"
	}
	return ""
}
