package policy

import (
	"fmt"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//ContainsVariablesOtherThanObject returns error if variable that does not start from request.object
func ContainsVariablesOtherThanObject(policy kyverno.ClusterPolicy) error {
	var err error
	for idx, rule := range policy.Spec.Rules {
		if path := userInfoDefined(rule.MatchResources.UserInfo); path != "" {
			return fmt.Errorf("invalid variable used at path: spec/rules[%d]/match/%s", idx, path)
		}

		if path := userInfoDefined(rule.ExcludeResources.UserInfo); path != "" {
			return fmt.Errorf("invalid variable used at path: spec/rules[%d]/exclude/%s", idx, path)
		}
		// Skip Validation if rule contains Context
		if len(rule.Context) > 0 {
			return nil
		}

		filterVars := []string{"request.object"}
		ctx := context.NewContext(filterVars...)
		for condIdx, condition := range rule.Conditions {
			if condition.Key, err = variables.SubstituteVars(log.Log, ctx, condition.Key); !checkNotFoundErr(err) {
				return fmt.Errorf("invalid variable %s used at spec/rules[%d]/condition[%d]/key", condition.Key, idx, condIdx)
			}

			if condition.Value, err = variables.SubstituteVars(log.Log, ctx, condition.Value); !checkNotFoundErr(err) {
				return fmt.Errorf("invalid %s variable used at spec/rules[%d]/condition[%d]/value", condition.Value, idx, condIdx)
			}
		}

		if rule.Mutation.Overlay != nil {
			if rule.Mutation.Overlay, err = variables.SubstituteVars(log.Log, ctx, rule.Mutation.Overlay); !checkNotFoundErr(err) {
				return fmt.Errorf("invalid variable used at spec/rules[%d]/mutate/overlay", idx)
			}
		}

		if rule.Validation.Pattern != nil {
			if rule.Validation.Pattern, err = variables.SubstituteVars(log.Log, ctx, rule.Validation.Pattern); !checkNotFoundErr(err) {
				return fmt.Errorf("invalid variable used at spec/rules[%d]/validate/pattern", idx)
			}
		}

		anyPattern, err := rule.Validation.DeserializeAnyPattern()
		if err != nil {
			return fmt.Errorf("failed to deserialze anyPattern, expect array: %v", err)
		}

		for idx2, pattern := range anyPattern {
			if anyPattern[idx2], err = variables.SubstituteVars(log.Log, ctx, pattern); !checkNotFoundErr(err) {
				return fmt.Errorf("invalid variable used at spec/rules[%d]/validate/anyPattern[%d]", idx, idx2)
			}
		}

		if _, err = variables.SubstituteVars(log.Log, ctx, rule.Validation.Message); !checkNotFoundErr(err) {
			return fmt.Errorf("invalid variable used at spec/rules[%d]/validate/message", idx)
		}

		if rule.Validation.Deny != nil {
			for i := range rule.Validation.Deny.Conditions {
				if _, err = variables.SubstituteVars(log.Log, ctx, rule.Validation.Deny.Conditions[i].Key); !checkNotFoundErr(err) {
					return fmt.Errorf("invalid variable %s used at spec/rules[%d]/validate/deny/conditions[%d]/key: %v",
						rule.Validation.Deny.Conditions[i].Key, idx, i, err)
				}
				if _, err = variables.SubstituteVars(log.Log, ctx, rule.Validation.Deny.Conditions[i].Value); !checkNotFoundErr(err) {
					return fmt.Errorf("invalid variable %s used at spec/rules[%d]/validate/deny/conditions[%d]/value: %v",
						rule.Validation.Deny.Conditions[i].Value, idx, i, err)
				}
			}
		}
	}

	return nil
}

func checkNotFoundErr(err error) bool {
	if err != nil {
		switch err.(type) {
		case variables.NotFoundVariableErr:
			return true
		default:
			return false
		}
	}

	return true
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
