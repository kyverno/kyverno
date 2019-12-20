package variables

import (
	"regexp"
)

func CheckVariables(pattern interface{}, variables []string) bool {
	switch typedPattern := pattern.(type) {
	case map[string]interface{}:
		return checkMap(typedPattern, variables)
	case []interface{}:
		return checkArray(typedPattern, variables)
	case string:
		return checkValue(typedPattern, variables)
	default:
		return false
	}
}

func checkMap(patternMap map[string]interface{}, variables []string) bool {
	for _, patternElement := range patternMap {
		exists := CheckVariables(patternElement, variables)
		if exists {
			return exists
		}
	}
	return false
}

func checkArray(patternList []interface{}, variables []string) bool {
	for _, patternElement := range patternList {
		exists := CheckVariables(patternElement, variables)
		if exists {
			return exists
		}
	}
	return false
}

func checkValue(valuePattern string, variables []string) bool {
	operatorVariable := getOperator(valuePattern)
	variable := valuePattern[len(operatorVariable):]
	return checkValueVariable(variable, variables)
}

func checkValueVariable(valuePattern string, variables []string) bool {
	variableRegex := regexp.MustCompile("^{{(.*)}}$")
	groups := variableRegex.FindStringSubmatch(valuePattern)
	if len(groups) < 2 {
		return false
	}
	return variablePatternSearch(groups[1], variables)
}

func variablePatternSearch(pattern string, regexs []string) bool {
	for _, regex := range regexs {
		varRegex := regexp.MustCompile(regex)
		found := varRegex.FindString(pattern)
		if found != "" {
			return true
		}
	}
	return true
}
