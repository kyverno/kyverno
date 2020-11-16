package variables

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/context"
)

const (
	variableRegex = `\{\{([^{}]*)\}\}`
)

//SubstituteVars replaces the variables with the values defined in the context
// - if any variable is invaid or has nil value, it is considered as a failed varable substitution
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
		return subValR(ctx, typedPattern, path)
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

type NotFoundVariableErr struct {
	variable string
	path     string
}

func (n NotFoundVariableErr) Error() string {
	return fmt.Sprintf("variable %v not found (path: %v)", n.variable, n.path)
}

// subValR resolves the variables if defined
func subValR(ctx context.EvalInterface, valuePattern string, path string) (interface{}, error) {
	originalPattern := valuePattern

	regex := regexp.MustCompile(`\{\{([^{}]*)\}\}`)
	for {
		if vars := regex.FindAllString(valuePattern, -1); len(vars) > 0 {
			for _, variable := range vars {
				underlyingVariable := strings.ReplaceAll(strings.ReplaceAll(variable, "}}", ""), "{{", "")
				substitutedVar, err := ctx.Query(underlyingVariable)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve %v at path %s", underlyingVariable, path)
				}
				if val, ok := substitutedVar.(string); ok {
					valuePattern = strings.Replace(valuePattern, variable, val, -1)
				} else {
					if substitutedVar != nil {
						if originalPattern == variable {
							return substitutedVar, nil
						}
						return nil, fmt.Errorf("failed to resolve %v at path %s", underlyingVariable, path)
					}
					return nil, NotFoundVariableErr{
						variable: underlyingVariable,
						path:     path,
					}
				}
			}
		} else {
			break
		}
	}

	return valuePattern, nil
}
