package policy

import (
	"encoding/json"
	"fmt"
	"strings"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//ContainsUserVariables returns error if variable that does not start from request.object
func containsUserVariables(policy *kyverno.ClusterPolicy, vars [][]string) error {
	for _, s := range vars {
		if strings.Contains(s[0], "userInfo") {
			return fmt.Errorf("variable %s is not allowed", s[0])
		}
	}

	for idx := range policy.Spec.Rules {
		if err := hasUserMatchExclude(idx, &policy.Spec.Rules[idx]); err != nil {
			return err
		}
	}

	return nil
}

func hasUserMatchExclude(idx int, rule *kyverno.Rule) error {
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
