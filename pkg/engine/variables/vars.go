package variables

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/context"
)

var regexVariables = regexp.MustCompile(`\{\{[^{}]*\}\}`)

//IsVariable returns true if the element contains a 'valid' variable {{}}
func IsVariable(element string) bool {
	groups := regexVariables.FindAllStringSubmatch(element, -1)
	return len(groups) != 0
}

//SubstituteVars replaces the variables with the values defined in the context
// - if any variable is invalid or has nil value, it is considered as a failed variable substitution
func SubstituteVars(log logr.Logger, ctx context.EvalInterface, pattern interface{}) (interface{}, error) {
	pattern, err := subVars(log, ctx, pattern, "")
	if err != nil {
		return pattern, err
	}
	return pattern, nil
}

func subVars(log logr.Logger, ctx context.EvalInterface, pattern interface{}, path string) (interface{}, error) {
	switch typedPattern := pattern.(type) {
	case map[string]interface{}:
		mapCopy := make(map[string]interface{})
		for k, v := range typedPattern {
			mapCopy[k] = v
		}
		return subMap(log, ctx, mapCopy, path)

	case []interface{}:
		sliceCopy := make([]interface{}, len(typedPattern))
		copy(sliceCopy, typedPattern)
		return subArray(log, ctx, sliceCopy, path)

	case string:
		return subValR(log, ctx, typedPattern, path)

	default:
		return pattern, nil
	}
}

func subMap(log logr.Logger, ctx context.EvalInterface, patternMap map[string]interface{}, path string) (map[string]interface{}, error) {
	for key, patternElement := range patternMap {
		curPath := path + "/" + key
		value, err := subVars(log, ctx, patternElement, curPath)
		if err != nil {
			return nil, err
		}
		patternMap[key] = value

	}
	return patternMap, nil
}

func subArray(log logr.Logger, ctx context.EvalInterface, patternList []interface{}, path string) ([]interface{}, error) {
	for idx, patternElement := range patternList {
		curPath := path + "/" + strconv.Itoa(idx)
		value, err := subVars(log, ctx, patternElement, curPath)
		if err != nil {
			return nil, err
		}
		patternList[idx] = value
	}
	return patternList, nil
}

// NotFoundVariableErr ...
type NotFoundVariableErr struct {
	variable string
	path     string
}

func (n NotFoundVariableErr) Error() string {
	return fmt.Sprintf("variable %s not resolved at path %s", n.variable, n.path)
}

// subValR resolves the variables if defined
func subValR(log logr.Logger, ctx context.EvalInterface, valuePattern string, path string) (interface{}, error) {
	originalPattern := valuePattern
	vars := regexVariables.FindAllString(valuePattern, -1)
	for len(vars) > 0 {
		for _, v := range vars {
			variable := strings.ReplaceAll(v, "{{", "")
			variable = strings.ReplaceAll(variable, "}}", "")
			variable = strings.TrimSpace(variable)
			substitutedVar, err := ctx.Query(variable)
			if err != nil {
				switch err.(type) {
				case context.InvalidVariableErr:
					return nil, err
				default:
					return nil, fmt.Errorf("failed to resolve %v at path %s", variable, path)
				}
			}

			log.V(3).Info("variable substituted", "variable", v, "value", substitutedVar, "path", path)

			if val, ok := substitutedVar.(string); ok {
				valuePattern = strings.Replace(valuePattern, v, val, -1)
				continue
			}

			if substitutedVar != nil {
				if originalPattern == v {
					return substitutedVar, nil
				}

				return nil, fmt.Errorf("failed to resolve %v at path %s", variable, path)
			}

			return nil, NotFoundVariableErr{
				variable: variable,
				path:     path,
			}
		}

		// check for nested variables in strings
		vars = regexVariables.FindAllString(valuePattern, -1)
	}

	return valuePattern, nil
}
