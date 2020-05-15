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
	variableRegex  = `\{\{([^{}]*)\}\}`
	singleVarRegex = `^\{\{([^{}]*)\}\}$`
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
		return subValR(log, ctx, typedPattern, path, errs)
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
func subValR(log logr.Logger, ctx context.EvalInterface, valuePattern string, path string, errs *[]error) interface{} {

	// variable values can be scalar values(string,int, float) or they can be obects(map,slice)
	// - {{variable}}
	// there is a single variable resolution so the value can be scalar or object
	// - {{variable1--{{variable2}}}}}
	// variable2 is evaluted first as an individual variable and can be have scalar or object values
	// but resolving the outer variable, {{variable--<value>}}
	// if <value> is scalar then it can replaced, but for object types its tricky
	// as object cannot be directy replaced, if the object is stringyfied then it loses it structure.
	// since this might be a potential place for error, required better error reporting and handling

	// object values are only suported for single variable substitution
	if ok, retVal := processIfSingleVariable(log, ctx, valuePattern, path, errs); ok {
		return retVal
	}
	// var emptyInterface interface{}
	var failedVars []string
	// process type string
	for {
		valueStr := valuePattern
		if len(failedVars) != 0 {
			log.Info("failed to resolve variablesl short-circuiting")
			break
		}
		// get variables at this level
		validRegex := regexp.MustCompile(variableRegex)
		groups := validRegex.FindAllStringSubmatch(valueStr, -1)
		if len(groups) == 0 {
			// there was no match
			// not variable defined
			break
		}
		subs := map[string]interface{}{}
		for _, group := range groups {
			if _, ok := subs[group[0]]; ok {
				// value has already been substituted
				continue
			}
			// here we do the querying of the variables from the context
			variable, err := ctx.Query(group[1])
			if err != nil {
				// error while evaluating
				failedVars = append(failedVars, group[1])
				continue
			}
			// path not found in context and value stored in null/nill
			if variable == nil {
				failedVars = append(failedVars, group[1])
				continue
			}
			// get values for each and replace
			subs[group[0]] = variable
		}
		// perform substitutions
		newVal := valueStr
		for k, v := range subs {
			// if value is of type string then cast else consider it as direct replacement
			if val, ok := v.(string); ok {
				newVal = strings.Replace(newVal, k, val, -1)
				continue
			}
			// if type is not scalar then consider this as a failed variable
			log.Info("variable resolves to non-scalar value. Non-Scalar values are not supported for nested variables", "variable", k, "value", v)
			failedVars = append(failedVars, k)
		}
		valuePattern = newVal
	}
	// update errors if any
	if len(failedVars) > 0 {
		*errs = append(*errs, fmt.Errorf("failed to resolve %v at path %s", failedVars, path))
	}

	return valuePattern
}

// processIfSingleVariable will process the evaluation of single variables
// {{variable-{{variable}}}} -> compound/nested variables
// {{variable}}{{variable}} -> multiple variables
// {{variable}} -> single variable
// if the value can be evaluted return the value
// -> return value can be scalar or object type
// -> if the variable is not present in the context then add an error and dont process further
func processIfSingleVariable(log logr.Logger, ctx context.EvalInterface, valuePattern interface{}, path string, errs *[]error) (bool, interface{}) {
	valueStr, ok := valuePattern.(string)
	if !ok {
		log.Info("failed to convert to string", "pattern", valuePattern)
		return false, nil
	}
	// get variables at this level
	validRegex := regexp.MustCompile(singleVarRegex)
	groups := validRegex.FindAllStringSubmatch(valueStr, -1)
	if len(groups) == 0 {
		return false, nil
	}
	// as there will be exactly one variable based on the above regex
	group := groups[0]
	variable, err := ctx.Query(group[1])
	if err != nil || variable == nil {
		*errs = append(*errs, fmt.Errorf("failed to resolve %v at path %s", group[1], path))
		// return the same value pattern, and add un-resolvable variable error
		return true, valuePattern
	}
	return true, variable
}
