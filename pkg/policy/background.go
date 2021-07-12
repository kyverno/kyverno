package policy

import (
	"encoding/json"
	"fmt"

	gojmespath "github.com/jmespath/go-jmespath"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/utils"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//ContainsVariablesOtherThanObject returns error if variable that does not start from request.object
func ContainsVariablesOtherThanObject(policy kyverno.ClusterPolicy, mock bool) error {
	var err error
	for idx, rule := range policy.Spec.Rules {
		if path := userInfoDefined(rule.MatchResources.UserInfo); path != "" {
			return fmt.Errorf("invalid variable used at path: spec/rules[%d]/match/%s", idx, path)
		}

		if path := userInfoDefined(rule.ExcludeResources.UserInfo); path != "" {
			return fmt.Errorf("invalid variable used at path: spec/rules[%d]/exclude/%s", idx, path)
		}

		filterVars := []string{"request.object", "request.namespace", "images"}
		ctx := context.NewContext(filterVars...)

		for _, contextEntry := range rule.Context {
			if contextEntry.APICall != nil {
				ctx.AddBuiltInVars(contextEntry.Name)
			}

			if contextEntry.ConfigMap != nil {
				ctx.AddBuiltInVars(contextEntry.Name)
			}
		}

		if !mock {
			err = validateBackgroundModeVars(ctx, rule)
			if err != nil {
				return err
			}
			if rule, err = variables.SubstituteAllInRule(log.Log, ctx, rule); !checkNotFoundErr(err) {
				return fmt.Errorf("variable substitution failed for rule %s: %s", rule.Name, err.Error())
			}
		}

		if rule.AnyAllConditions != nil {
			if err = validatePreConditions(idx, ctx, rule.AnyAllConditions, mock); !checkNotFoundErr(err) {
				return err
			}
		}

		if rule.Validation.Deny != nil {
			if err = validateDenyConditions(idx, ctx, rule.Validation.Deny.AnyAllConditions, mock); !checkNotFoundErr(err) {
				return err
			}
		}

	}

	return nil
}

func validatePreConditions(idx int, ctx context.EvalInterface, anyAllConditions apiextensions.JSON, mock bool) error {
	var err error

	if !mock {
		anyAllConditions, err = substituteVarsInJSON(ctx, anyAllConditions)
		if err != nil {
			return err
		}
	}

	_, err = utils.ApiextensionsJsonToKyvernoConditions(anyAllConditions)
	if err != nil {
		return err
	}

	return nil
}

func validateDenyConditions(idx int, ctx context.EvalInterface, denyConditions apiextensions.JSON, mock bool) error {
	var err error

	if !mock {
		denyConditions, err = substituteVarsInJSON(ctx, denyConditions)
		if err != nil {
			return err
		}
	}

	_, err = utils.ApiextensionsJsonToKyvernoConditions(denyConditions)
	if err != nil {
		return err
	}

	return nil
}

func checkNotFoundErr(err error) bool {
	if err != nil {
		switch err.(type) {
		case gojmespath.NotFoundError:
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

func validateBackgroundModeVars(ctx context.EvalInterface, document apiextensions.JSON) error {
	jsonByte, err := json.Marshal(document)
	if err != nil {
		return err
	}

	var jsonInterface interface{}
	err = json.Unmarshal(jsonByte, &jsonInterface)
	if err != nil {
		return err
	}

	_, err = variables.ValidateBackgroundModeVars(log.Log, ctx, jsonInterface)
	return err
}

func substituteVarsInJSON(ctx context.EvalInterface, document apiextensions.JSON) (apiextensions.JSON, error) {
	jsonByte, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}

	var jsonInterface interface{}
	err = json.Unmarshal(jsonByte, &jsonInterface)
	if err != nil {
		return nil, err
	}

	jsonInterface, err = variables.SubstituteAll(log.Log, ctx, jsonInterface)
	if err != nil {
		return nil, err
	}

	jsonByte, err = json.Marshal(jsonInterface)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(jsonByte, &document)
	if err != nil {
		return nil, err
	}

	return document, nil
}
