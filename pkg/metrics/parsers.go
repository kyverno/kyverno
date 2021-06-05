package metrics

import (
	"fmt"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"reflect"
)

func ParsePolicyValidationMode(validationFailureAction string) (PolicyValidationMode, error) {
	switch validationFailureAction {
	case "enforce":
		return Enforce, nil
	case "audit":
		return Audit, nil
	default:
		return "", fmt.Errorf("wrong validation failure action found %s. Allowed: '%s', '%s'", validationFailureAction, "enforce", "audit")
	}
}

func ParsePolicyBackgroundMode(backgroundMode bool) PolicyBackgroundMode {
	if backgroundMode {
		return BackgroundTrue
	}
	return BackgroundFalse
}

func ParseRuleType(rule kyverno.Rule) RuleType {
	if !reflect.DeepEqual(rule.Validation, kyverno.Validation{}) {
		return Validate
	}
	if !reflect.DeepEqual(rule.Mutation, kyverno.Mutation{}) {
		return Mutate
	}
	if !reflect.DeepEqual(rule.Generation, kyverno.Generation{}) {
		return Generate
	}
	return EmptyRuleType
}
