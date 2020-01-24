package variables

import (
	"regexp"
	"strings"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/operator"
)

const variableRegex = `\{\{([^{}]*)\}\}`

//SubstituteVariables substitutes the JMESPATH with variable substitution
// supported substitutions
// - no operator + variable(string,object)
// unsupported substitutions
// - operator + variable(object) -> as we dont support operators with object types
func SubstituteVariables(ctx context.EvalInterface, pattern interface{}) interface{} {
	// var err error
	switch typedPattern := pattern.(type) {
	case map[string]interface{}:
		return substituteMap(ctx, typedPattern)
	case []interface{}:
		return substituteArray(ctx, typedPattern)
	case string:
		// variable substitution
		return substituteValue(ctx, typedPattern)
	default:
		return pattern
	}
}

func substituteMap(ctx context.EvalInterface, patternMap map[string]interface{}) map[string]interface{} {
	for key, patternElement := range patternMap {
		value := SubstituteVariables(ctx, patternElement)
		patternMap[key] = value
	}
	return patternMap
}

func substituteArray(ctx context.EvalInterface, patternList []interface{}) []interface{} {
	for idx, patternElement := range patternList {
		value := SubstituteVariables(ctx, patternElement)
		patternList[idx] = value
	}
	return patternList
}

func substituteValue(ctx context.EvalInterface, valuePattern string) interface{} {
	// patterns supported
	// - operator + string
	// operator + variable
	operatorVariable := getOperator(valuePattern)
	variable := valuePattern[len(operatorVariable):]
	// substitute variable with value
	value := getValueQuery(ctx, variable)
	if operatorVariable == "" {
		// default or operator.Equal
		// equal + string variable
		// object variable
		return value
	}
	// operator + string variable
	switch value.(type) {
	case string:
		return string(operatorVariable) + value.(string)
	default:
		glog.Infof("cannot use operator with object variables. operator used %s in pattern %v", string(operatorVariable), valuePattern)
		var emptyInterface interface{}
		return emptyInterface
	}
}

func getValueQuery(ctx context.EvalInterface, valuePattern string) interface{} {
	var emptyInterface interface{}
	// extract variable {{<variable>}}
	validRegex := regexp.MustCompile(variableRegex)
	groups := validRegex.FindAllStringSubmatch(valuePattern, -1)
	// can have multiple variables in a single value pattern
	// var Map <variable,value>
	varMap := getValues(ctx, groups)
	if len(varMap) == 0 {
		// there are no varaiables
		// return the original value
		return valuePattern
	}
	// only substitute values if all the variable values are of type string
	if isAllVarStrings(varMap) {
		newVal := valuePattern
		for key, value := range varMap {
			if val, ok := value.(string); ok {
				newVal = strings.Replace(newVal, key, val, -1)
			}
		}
		return newVal
	}

	// we do not support multiple substitution per statement for non-string types
	for _, value := range varMap {
		return value
	}
	return emptyInterface
}

// returns map of variables as keys and variable values as values
func getValues(ctx context.EvalInterface, groups [][]string) map[string]interface{} {
	var emptyInterface interface{}
	subs := map[string]interface{}{}
	for _, group := range groups {
		if len(group) == 2 {
			// 0th is string
			varName := group[0]
			varValue := group[1]
			variable, err := ctx.Query(varValue)
			if err != nil {
				glog.V(4).Infof("variable substitution failed for query %s: %v", varName, err)
				subs[varName] = emptyInterface
				continue
			}
			if variable == nil {
				subs[varName] = emptyInterface
			} else {
				subs[varName] = variable
			}
		}
	}
	return subs
}

func isAllVarStrings(subVar map[string]interface{}) bool {
	for _, value := range subVar {
		if _, ok := value.(string); !ok {
			return false
		}
	}
	return true
}

func getOperator(pattern string) string {
	operatorVariable := operator.GetOperatorFromStringPattern(pattern)
	if operatorVariable == operator.Equal {
		return ""
	}
	return string(operatorVariable)
}
