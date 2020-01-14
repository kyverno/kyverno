package variables

import (
	"regexp"
	"strings"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/engine/context"
)

// ValidateVariables validates if referenced path is present
// return empty string if all paths are valid, otherwise return invalid path
func ValidateVariables(ctx context.EvalInterface, pattern interface{}) string {
	var pathsNotPresent []string
	variableList := extractVariables(pattern)
	for _, variable := range variableList {
		if len(variable) == 2 {
			varName := variable[0]
			varValue := variable[1]
			glog.V(3).Infof("validating variable %s", varName)
			val, err := ctx.Query(varValue)
			if err == nil && val == nil {
				// path is not present, returns nil interface
				pathsNotPresent = append(pathsNotPresent, varValue)
			}
		}
	}

	if len(pathsNotPresent) != 0 {
		return strings.Join(pathsNotPresent, ";")
	}
	return ""
}

// extractVariables extracts variables in the pattern
func extractVariables(pattern interface{}) [][]string {
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

func extractMap(patternMap map[string]interface{}) [][]string {
	var variableList [][]string

	for _, patternElement := range patternMap {
		if vars := extractVariables(patternElement); vars != nil {
			variableList = append(variableList, vars...)
		}
	}
	return variableList
}

func extractArray(patternList []interface{}) [][]string {
	var variableList [][]string

	for _, patternElement := range patternList {
		if vars := extractVariables(patternElement); vars != nil {
			variableList = append(variableList, vars...)
		}
	}
	return variableList
}

func extractValue(valuePattern string) [][]string {
	operatorVariable := getOperator(valuePattern)
	variable := valuePattern[len(operatorVariable):]
	return extractValueVariable(variable)
}

// returns multiple variable match groups
func extractValueVariable(valuePattern string) [][]string {
	variableRegex := regexp.MustCompile(variableRegex)
	groups := variableRegex.FindAllStringSubmatch(valuePattern, -1)
	if len(groups) == 0 {
		// no variables
		return nil
	}
	// group[*] <- all the matches
	// group[*][0] <- group match
	// group[*][1] <- group capture value
	return groups
}
