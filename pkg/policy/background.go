package policy

import (
	"fmt"
	"reflect"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/utils"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
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

		filterVars := []string{"request.object", "request.namespace"}
		ctx := context.NewContext(filterVars...)

		for contextIdx, contextEntry := range rule.Context {
			if contextEntry.APICall != nil {
				ctx.AddBuiltInVars(contextEntry.Name)

				if _, err := variables.SubstituteVars(log.Log, ctx, contextEntry.APICall.URLPath); !checkNotFoundErr(err) {
					return fmt.Errorf("invalid variable used at spec/rules[%d]/context[%d]/apiCall/urlPath: %s", idx, contextIdx, err.Error())
				}

				if _, err := variables.SubstituteVars(log.Log, ctx, contextEntry.APICall.JMESPath); !checkNotFoundErr(err) {
					return fmt.Errorf("invalid variable used at spec/rules[%d]/context[%d]/apiCall/jmesPath: %s", idx, contextIdx, err.Error())
				}
			}

			if contextEntry.ConfigMap != nil {
				ctx.AddBuiltInVars(contextEntry.Name)

				if _, err = variables.SubstituteVars(log.Log, ctx, contextEntry.ConfigMap.Name); !checkNotFoundErr(err) {
					return fmt.Errorf("invalid variable used at spec/rules[%d]/context[%d]/configMap/name: %s", idx, contextIdx, err.Error())
				}

				if _, err = variables.SubstituteVars(log.Log, ctx, contextEntry.ConfigMap.Namespace); !checkNotFoundErr(err) {
					return fmt.Errorf("invalid variable used at spec/rules[%d]/context[%d]/configMap/namespace: %s", idx, contextIdx, err.Error())
				}
			}
		}

		if rule.AnyAllConditions != nil {
			if err = validatePreConditions(idx, ctx, rule.AnyAllConditions); err != nil {
				return err
			}
		}

		if rule.Mutation.Overlay != nil {
			if rule.Mutation.Overlay, err = variables.SubstituteVars(log.Log, ctx, rule.Mutation.Overlay); !checkNotFoundErr(err) {
				return fmt.Errorf("invalid variable used at spec/rules[%d]/mutate/overlay: %s", idx, err.Error())
			}
		}

		if rule.Mutation.PatchStrategicMerge != nil {
			if rule.Mutation.Overlay, err = variables.SubstituteVars(log.Log, ctx, rule.Mutation.PatchStrategicMerge); !checkNotFoundErr(err) {
				return fmt.Errorf("invalid variable used at spec/rules[%d]/mutate/patchStrategicMerge: %s", idx, err.Error())
			}
		}

		if rule.Validation.Pattern != nil {
			if rule.Validation.Pattern, err = variables.SubstituteVars(log.Log, ctx, rule.Validation.Pattern); !checkNotFoundErr(err) {
				return fmt.Errorf("invalid variable used at spec/rules[%d]/validate/pattern: %s", idx, err.Error())
			}
		}

		anyPattern, err := rule.Validation.DeserializeAnyPattern()
		if err != nil {
			return fmt.Errorf("failed to deserialize anyPattern, expect array: %s", err.Error())
		}

		for idx2, pattern := range anyPattern {
			if anyPattern[idx2], err = variables.SubstituteVars(log.Log, ctx, pattern); !checkNotFoundErr(err) {
				return fmt.Errorf("invalid variable used at spec/rules[%d]/validate/anyPattern[%d]: %s", idx, idx2, err.Error())
			}
		}

		if _, err = variables.SubstituteVars(log.Log, ctx, rule.Validation.Message); !checkNotFoundErr(err) {
			return fmt.Errorf("invalid variable used at spec/rules[%d]/validate/message: %s", idx, err.Error())
		}

		if rule.Validation.Deny != nil {
			if err = validateDenyConditions(idx, ctx, rule.Validation.Deny.AnyAllConditions); err != nil {
				return err
			}
		}

		if _, err = variables.SubstituteVars(log.Log, ctx, rule.Generation.Name); !checkNotFoundErr(err) {
			return fmt.Errorf("invalid variable used at spec/rules[%d]/generate/name: %v", idx, err)
		}

		if _, err = variables.SubstituteVars(log.Log, ctx, rule.Generation.Namespace); !checkNotFoundErr(err) {
			return fmt.Errorf("invalid variable used at spec/rules[%d]/generate/name: %v", idx, err)
		}

		if _, err = variables.SubstituteVars(log.Log, ctx, rule.Generation.Data); !checkNotFoundErr(err) {
			return fmt.Errorf("invalid variable used at spec/rules[%d]/generate/data: %v", idx, err)
		}

		if _, err = variables.SubstituteVars(log.Log, ctx, rule.Generation.Clone.Name); !checkNotFoundErr(err) {
			return fmt.Errorf("invalid variable used at spec/rules[%d]/generate/clone/name: %v", idx, err)
		}

		if _, err = variables.SubstituteVars(log.Log, ctx, rule.Generation.Clone.Namespace); !checkNotFoundErr(err) {
			return fmt.Errorf("invalid variable used at spec/rules[%d]/generate/clone/namespace: %v", idx, err)
		}
	}

	return nil
}

func validatePreConditions(idx int, ctx context.EvalInterface, anyAllConditions apiextensions.JSON) error {
	var err error
	// conditions are currently in the form of []interface{}
	kyvernoAnyAllConditions, err := utils.ApiextensionsJsonToKyvernoConditions(anyAllConditions)
	if err != nil {
		return err
	}
	switch typedPreConditions := kyvernoAnyAllConditions.(type) {
	case kyverno.AnyAllConditions:
		if !reflect.DeepEqual(typedPreConditions, kyverno.AnyAllConditions{}) && typedPreConditions.AnyConditions != nil {
			for condIdx, condition := range typedPreConditions.AnyConditions {
				if condition.Key, err = variables.SubstituteVars(log.Log, ctx, condition.Key); !checkNotFoundErr(err) {
					return fmt.Errorf("invalid variable %s used at spec/rules[%d]/any/condition[%d]/key", condition.Key, idx, condIdx)
				}

				if condition.Value, err = variables.SubstituteVars(log.Log, ctx, condition.Value); !checkNotFoundErr(err) {
					return fmt.Errorf("invalid %s variable used at spec/rules[%d]/any/condition[%d]/value", condition.Value, idx, condIdx)
				}
			}
		}
		if !reflect.DeepEqual(typedPreConditions, kyverno.AnyAllConditions{}) && typedPreConditions.AllConditions != nil {
			for condIdx, condition := range typedPreConditions.AllConditions {
				if condition.Key, err = variables.SubstituteVars(log.Log, ctx, condition.Key); !checkNotFoundErr(err) {
					return fmt.Errorf("invalid variable %s used at spec/rules[%d]/all/condition[%d]/key", condition.Key, idx, condIdx)
				}

				if condition.Value, err = variables.SubstituteVars(log.Log, ctx, condition.Value); !checkNotFoundErr(err) {
					return fmt.Errorf("invalid %s variable used at spec/rules[%d]/all/condition[%d]/value", condition.Value, idx, condIdx)
				}
			}
		}
	case []kyverno.Condition: //backwards compatibility
		for condIdx, condition := range typedPreConditions {
			if condition.Key, err = variables.SubstituteVars(log.Log, ctx, condition.Key); !checkNotFoundErr(err) {
				return fmt.Errorf("invalid variable %s used at spec/rules[%d]/condition[%d]/key", condition.Key, idx, condIdx)
			}

			if condition.Value, err = variables.SubstituteVars(log.Log, ctx, condition.Value); !checkNotFoundErr(err) {
				return fmt.Errorf("invalid %s variable used at spec/rules[%d]/condition[%d]/value", condition.Value, idx, condIdx)
			}
		}
	}

	return nil
}

func validateDenyConditions(idx int, ctx context.EvalInterface, denyConditions apiextensions.JSON) error {
	// conditions are currently in the form of []interface{}
	kyvernoDenyConditions, err := utils.ApiextensionsJsonToKyvernoConditions(denyConditions)
	if err != nil {
		return err
	}
	switch typedDenyConditions := kyvernoDenyConditions.(type) {
	case kyverno.AnyAllConditions:
		// validating validate.deny.any.conditions
		if !reflect.DeepEqual(typedDenyConditions, kyverno.AnyAllConditions{}) && typedDenyConditions.AnyConditions != nil {
			for i := range typedDenyConditions.AnyConditions {
				if _, err := variables.SubstituteVars(log.Log, ctx, typedDenyConditions.AnyConditions[i].Key); !checkNotFoundErr(err) {
					return fmt.Errorf("invalid variable %s used at spec/rules[%d]/validate/deny/any/conditions[%d]/key: %v",
						typedDenyConditions.AnyConditions[i].Key, idx, i, err)
				}
				if _, err := variables.SubstituteVars(log.Log, ctx, typedDenyConditions.AnyConditions[i].Value); !checkNotFoundErr(err) {
					return fmt.Errorf("invalid variable %s used at spec/rules[%d]/validate/deny/any/conditions[%d]/value: %v",
						typedDenyConditions.AnyConditions[i].Value, idx, i, err)
				}
			}
		}
		// validating validate.deny.all.conditions
		if !reflect.DeepEqual(typedDenyConditions, kyverno.AnyAllConditions{}) && typedDenyConditions.AllConditions != nil {
			for i := range typedDenyConditions.AllConditions {
				if _, err := variables.SubstituteVars(log.Log, ctx, typedDenyConditions.AllConditions[i].Key); !checkNotFoundErr(err) {
					return fmt.Errorf("invalid variable %s used at spec/rules[%d]/validate/deny/all/conditions[%d]/key: %v",
						typedDenyConditions.AllConditions[i].Key, idx, i, err)
				}
				if _, err := variables.SubstituteVars(log.Log, ctx, typedDenyConditions.AllConditions[i].Value); !checkNotFoundErr(err) {
					return fmt.Errorf("invalid variable %s used at spec/rules[%d]/validate/deny/all/conditions[%d]/value: %v",
						typedDenyConditions.AllConditions[i].Value, idx, i, err)
				}
			}
		}
	case []kyverno.Condition: // backwards compatibility
		// validating validate.deny.conditions
		for i := range typedDenyConditions {
			if _, err := variables.SubstituteVars(log.Log, ctx, typedDenyConditions[i].Key); !checkNotFoundErr(err) {
				return fmt.Errorf("invalid variable %s used at spec/rules[%d]/validate/deny/conditions[%d]/key: %v",
					typedDenyConditions[i].Key, idx, i, err)
			}
			if _, err := variables.SubstituteVars(log.Log, ctx, typedDenyConditions[i].Value); !checkNotFoundErr(err) {
				return fmt.Errorf("invalid variable %s used at spec/rules[%d]/validate/deny/conditions[%d]/value: %v",
					typedDenyConditions[i].Value, idx, i, err)
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
		case context.InvalidVariableErr:
			// non-white-listed variable is found
			return false
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
