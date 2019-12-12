package variables

import (
	"regexp"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/operator"
)

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
		glog.Infof("cannot user operator with object variables. operator used %s in pattern %v", string(operatorVariable), valuePattern)
		var emptyInterface interface{}
		return emptyInterface
	}
}
func getValueQuery(ctx context.EvalInterface, valuePattern string) interface{} {
	var emptyInterface interface{}
	// extract variable {{<variable>}}
	variableRegex := regexp.MustCompile("^{{(.*)}}$")
	groups := variableRegex.FindStringSubmatch(valuePattern)
	if len(groups) < 2 {
		return valuePattern
	}
	searchPath := groups[1]
	// search for the path in ctx
	variable, err := ctx.Query(searchPath)
	if err != nil {
		glog.V(4).Infof("variable substituion failed for query %s: %v", searchPath, err)
		return emptyInterface
	}
	return variable
}

func getOperator(pattern string) string {
	operatorVariable := operator.GetOperatorFromStringPattern(pattern)
	if operatorVariable == operator.Equal {
		return ""
	}
	return string(operatorVariable)
}
