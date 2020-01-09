package variables

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/nirmata/kyverno/pkg/engine/context"
)

// ValidateVariables validates if referenced path is present
func ValidateVariables(ctx context.EvalInterface, pattern interface{}) error {
	var pathsNotPresent []string
	variableList := extractVariables(pattern)
	for i := 0; i < len(variableList)-1; i = i + 2 {
		p := variableList[i+1]
		val, err := ctx.Query(p)
		// reference path is not present
		if err == nil && val == nil {
			pathsNotPresent = append(pathsNotPresent, p)
		}
	}

	if len(variableList) != 0 && len(pathsNotPresent) != 0 {
		return fmt.Errorf("referenced paths are not present: %s", strings.Join(pathsNotPresent, ";"))
	}

	return nil
}

// extractVariables extracts variables in the pattern
func extractVariables(pattern interface{}) []string {
	switch typedPattern := pattern.(type) {
	case map[string]interface{}:
		return extractMap(typedPattern)
	case []interface{}:
		return extractArray(typedPattern)
	case string:
		return extractValue(typedPattern)
	default:
		return nil
	}
}

func extractMap(patternMap map[string]interface{}) []string {
	var variableList []string

	for _, patternElement := range patternMap {
		if vars := extractVariables(patternElement); vars != nil {
			variableList = append(variableList, vars...)
		}
	}
	return variableList
}

func extractArray(patternList []interface{}) []string {
	var variableList []string

	for _, patternElement := range patternList {
		if vars := extractVariables(patternElement); vars != nil {
			variableList = append(variableList, vars...)
		}
	}
	return variableList
}

func extractValue(valuePattern string) []string {
	operatorVariable := getOperator(valuePattern)
	variable := valuePattern[len(operatorVariable):]
	return extractValueVariable(variable)
}

func extractValueVariable(valuePattern string) []string {
	variableRegex := regexp.MustCompile(variableRegex)
	groups := variableRegex.FindStringSubmatch(valuePattern)
	if len(groups)%2 != 0 {
		return nil
	}
	return groups
}
