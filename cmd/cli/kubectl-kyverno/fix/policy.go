package fix

import (
	"fmt"
	"reflect"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
)

func FixPolicy(policy kyvernov1.PolicyInterface) ([]string, error) {
	var messages []string
	spec := policy.GetSpec()
	if spec.ValidationFailureAction.Enforce() {
		spec.ValidationFailureAction = kyvernov1.Enforce
	} else {
		spec.ValidationFailureAction = kyvernov1.Audit
	}
	for i := range spec.Rules {
		rule := &spec.Rules[i]
		if !reflect.DeepEqual(rule.MatchResources.ResourceDescription, kyvernov1.ResourceDescription{}) || !reflect.DeepEqual(rule.MatchResources.UserInfo, kyvernov1.UserInfo{}) {
			messages = append(messages, "match uses old syntax, moving to any")
			rule.MatchResources.Any = append(rule.MatchResources.Any, kyvernov1.ResourceFilter{
				ResourceDescription: rule.MatchResources.ResourceDescription,
				UserInfo:            rule.MatchResources.UserInfo,
			})
			rule.MatchResources.ResourceDescription = kyvernov1.ResourceDescription{}
			rule.MatchResources.UserInfo = kyvernov1.UserInfo{}
		}
		if !reflect.DeepEqual(rule.ExcludeResources.ResourceDescription, kyvernov1.ResourceDescription{}) || !reflect.DeepEqual(rule.ExcludeResources.UserInfo, kyvernov1.UserInfo{}) {
			messages = append(messages, "exclude uses old syntax, moving to any")
			rule.ExcludeResources.Any = append(rule.ExcludeResources.Any, kyvernov1.ResourceFilter{
				ResourceDescription: rule.ExcludeResources.ResourceDescription,
				UserInfo:            rule.ExcludeResources.UserInfo,
			})
			rule.ExcludeResources.ResourceDescription = kyvernov1.ResourceDescription{}
			rule.ExcludeResources.UserInfo = kyvernov1.UserInfo{}
		}
		preconditions := rule.GetAnyAllConditions()
		if preconditions != nil {
			cond, err := apiutils.ApiextensionsJsonToKyvernoConditions(preconditions)
			if err != nil {
				return messages, err
			}
			var newCond *kyvernov1.AnyAllConditions
			switch typedValue := cond.(type) {
			case kyvernov1.AnyAllConditions:
				newCond = &typedValue
			case []kyvernov1.Condition: // backwards compatibility
				newCond = &kyvernov1.AnyAllConditions{
					AllConditions: typedValue,
				}
			default:
				return messages, fmt.Errorf("unknown preconditions type: %T", typedValue)
			}
			fixCondition := func(c *kyvernov1.Condition) {
				switch c.Operator {
				case "Equal":
					messages = append(messages, "condition uses old operator `Equal`, updating")
					c.Operator = "Equals"
				case "NotEqual":
					messages = append(messages, "condition uses old operator `NotEqual`, updating")
					c.Operator = "NotEquals"
				case "In":
					messages = append(messages, "condition uses old operator `In`, updating")
					c.Operator = "AllIn"
				case "NotIn":
					messages = append(messages, "condition uses old operator `NotIn`, updating")
					c.Operator = "AnyNotIn"
				}
			}
			for c := range newCond.AnyConditions {
				fixCondition(&newCond.AnyConditions[c])
			}
			for c := range newCond.AllConditions {
				fixCondition(&newCond.AllConditions[c])
			}
			rule.SetAnyAllConditions(newCond)
		}
	}
	return messages, nil
}
