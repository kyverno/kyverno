package operator

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/utils/strings/slices"
)

var deprecatedOperators = map[string][]string{
	"In":    {"AllIn", "AnyIn"},
	"NotIn": {"AllNotIn", "AnyNotIn"},
}

func GetAllConditionOperators() []string {
	operators := make([]string, 0, len(kyvernov1.ConditionOperators))
	for k := range kyvernov1.ConditionOperators {
		operators = append(operators, k)
	}
	return operators
}

func GetAllDeprecatedOperators() []string {
	operators := make([]string, 0, len(deprecatedOperators))
	for k := range deprecatedOperators {
		operators = append(operators, k)
	}
	return operators
}

func GetDeprecatedOperatorAlternative(op string) []string {
	alts, ok := deprecatedOperators[op]
	if !ok {
		arr := make([]string, 0)
		return arr
	}
	return alts
}

func IsOperatorValid(operator kyvernov1.ConditionOperator) bool {
	return slices.Contains(GetAllConditionOperators(), string(operator))
}

func IsOperatorDeprecated(operator kyvernov1.ConditionOperator) bool {
	return slices.Contains(GetAllDeprecatedOperators(), string(operator))
}
