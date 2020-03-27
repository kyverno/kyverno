package policy

import (
	"fmt"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/variables"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//ContainsUserInfo returns error is userInfo is defined
func ContainsUserInfo(policy kyverno.ClusterPolicy) error {
	var err error
	// iterate of the policy rules to identify if userInfo is used
	for idx, rule := range policy.Spec.Rules {
		if path := userInfoDefined(rule.MatchResources.UserInfo); path != "" {
			return fmt.Errorf("userInfo variable used at path: spec/rules[%d]/match/%s", idx, path)
		}

		if path := userInfoDefined(rule.ExcludeResources.UserInfo); path != "" {
			return fmt.Errorf("userInfo variable used at path: spec/rules[%d]/exclude/%s", idx, path)
		}

		// variable defined with user information
		// - condition.key
		// - condition.value
		// - mutate.overlay
		// - validate.pattern
		// - validate.anyPattern[*]
		// variables to filter
		// - request.userInfo*
		// - serviceAccountName
		// - serviceAccountNamespace

		filterVars := []string{"request.userInfo*", "serviceAccountName", "serviceAccountNamespace"}
		ctx := context.NewContext(filterVars...)
		for condIdx, condition := range rule.Conditions {
			if condition.Key, err = variables.SubstituteVars(log.Log, ctx, condition.Key); err != nil {
				return fmt.Errorf("userInfo variable used at spec/rules[%d]/condition[%d]/key", idx, condIdx)
			}

			if condition.Value, err = variables.SubstituteVars(log.Log, ctx, condition.Value); err != nil {
				return fmt.Errorf("userInfo variable used at spec/rules[%d]/condition[%d]/value", idx, condIdx)
			}
		}

		if rule.Mutation.Overlay, err = variables.SubstituteVars(log.Log, ctx, rule.Mutation.Overlay); err != nil {
			return fmt.Errorf("userInfo variable used at spec/rules[%d]/mutate/overlay", idx)
		}
		if rule.Validation.Pattern, err = variables.SubstituteVars(log.Log, ctx, rule.Validation.Pattern); err != nil {
			return fmt.Errorf("userInfo variable used at spec/rules[%d]/validate/pattern", idx)
		}
		for idx2, pattern := range rule.Validation.AnyPattern {
			if rule.Validation.AnyPattern[idx2], err = variables.SubstituteVars(log.Log, ctx, pattern); err != nil {
				return fmt.Errorf("userInfo variable used at spec/rules[%d]/validate/anyPattern[%d]", idx, idx2)
			}
		}
	}
	return nil
}

func userInfoDefined(ui kyverno.UserInfo) string {
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
