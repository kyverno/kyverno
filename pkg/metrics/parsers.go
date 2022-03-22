package metrics

import (
	"fmt"
	"reflect"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
)

func ParsePolicyValidationMode(validationFailureAction kyverno.ValidationFailureAction) (PolicyValidationMode, error) {
	switch validationFailureAction {
	case kyverno.Enforce:
		return Enforce, nil
	case kyverno.Audit:
		return Audit, nil
	default:
		return "", fmt.Errorf("wrong validation failure action found %s. Allowed: '%s', '%s'", validationFailureAction, "enforce", "audit")
	}
}

func ParsePolicyBackgroundMode(backgroundMode *bool) PolicyBackgroundMode {
	if backgroundMode == nil || *backgroundMode {
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
