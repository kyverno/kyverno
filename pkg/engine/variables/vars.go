package variables

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/nirmata/kyverno/pkg/engine/context"
)

const (
	variableRegex = `\{\{([^{}]*)\}\}`
)

//SubstituteVars replaces the variables with the values defined in the context
// - if any variable is invaid or has nil value, it is considered as a failed varable substitution
func SubstituteVars(log logr.Logger, ctx context.EvalInterface, pattern interface{}) (interface{}, error) {
	errs := []error{}
	pattern = subVars(log, ctx, pattern, "", &errs)
	if len(errs) == 0 {
		// no error while parsing the pattern
		return pattern, nil
	}
	return pattern, fmt.Errorf("%v", errs)
}

func subVars(log logr.Logger, ctx context.EvalInterface, pattern interface{}, path string, errs *[]error) interface{} {
	switch typedPattern := pattern.(type) {
	case map[string]interface{}:
		mapCopy := make(map[string]interface{})
		for k, v := range typedPattern {
			mapCopy[k] = v
		}

		return subMap(log, ctx, mapCopy, path, errs)
	case []interface{}:
		sliceCopy := make([]interface{}, len(typedPattern))
		copy(sliceCopy, typedPattern)

		return subArray(log, ctx, sliceCopy, path, errs)
	case string:
		return subValR(ctx, typedPattern, path, errs)
	default:
		return pattern
	}
}

func subMap(log logr.Logger, ctx context.EvalInterface, patternMap map[string]interface{}, path string, errs *[]error) map[string]interface{} {
	for key, patternElement := range patternMap {
		curPath := path + "/" + key
		value := subVars(log, ctx, patternElement, curPath, errs)
		patternMap[key] = value

	}
	return patternMap
}

func subArray(log logr.Logger, ctx context.EvalInterface, patternList []interface{}, path string, errs *[]error) []interface{} {
	for idx, patternElement := range patternList {
		curPath := path + "/" + strconv.Itoa(idx)
		value := subVars(log, ctx, patternElement, curPath, errs)
		patternList[idx] = value
	}
	return patternList
}

// subValR resolves the variables if defined
func subValR(ctx context.EvalInterface, valuePattern string, path string, errs *[]error) interface{} {
	originalPattern := valuePattern
	var failedVars []interface{}

	defer func() {
		if len(failedVars) > 0 {
			*errs = append(*errs, fmt.Errorf("failed to resolve %v at path %s", failedVars, path))
		}
	}()

	regex := regexp.MustCompile(`\{\{([^{}]*)\}\}`)
	for {
		if vars := regex.FindAllString(valuePattern, -1); len(vars) > 0 {
			for _, variable := range vars {
				underlyingVariable := strings.ReplaceAll(strings.ReplaceAll(variable, "}}", ""), "{{", "")
				substitutedVar, err := ctx.Query(underlyingVariable)
				if err != nil {
					failedVars = append(failedVars, underlyingVariable)
					return nil
				}
				if val, ok := substitutedVar.(string); ok {
					if val == "" {
						failedVars = append(failedVars, underlyingVariable)
						return nil
					}
					valuePattern = strings.Replace(valuePattern, variable, val, -1)
				} else {
					if substitutedVar != nil {
						if originalPattern == variable {
							return substitutedVar
						}
					}
					failedVars = append(failedVars, underlyingVariable)
					return nil
				}
			}
		} else {
			break
		}
	}

	return valuePattern
}
