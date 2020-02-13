package variables

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/operator"
)

//SubstituteVars replaces the variables with the values defined in the context
// - if any variable is invaid or has nil value, it is considered as a failed varable substitution
func SubstituteVars(ctx context.EvalInterface, pattern interface{}) error {
	errs := []error{}
	subVars(ctx, pattern, "", &errs)
	if len(errs) == 0 {
		// no error while parsing the pattern
		return nil
	}
	return fmt.Errorf("variable(s) not found or has nil values: %v", errs)
}

func subVars(ctx context.EvalInterface, pattern interface{}, path string, errs *[]error) interface{} {
	switch typedPattern := pattern.(type) {
	case map[string]interface{}:
		return subMap(ctx, typedPattern, path, errs)
	case []interface{}:
		return subArray(ctx, typedPattern, path, errs)
	case string:
		return subVal(ctx, typedPattern, path, errs)
	default:
		return pattern
	}
}

func subMap(ctx context.EvalInterface, patternMap map[string]interface{}, path string, errs *[]error) map[string]interface{} {
	for key, patternElement := range patternMap {
		curPath := path + "/" + key
		value := subVars(ctx, patternElement, curPath, errs)
		patternMap[key] = value

	}
	return patternMap
}

func subArray(ctx context.EvalInterface, patternList []interface{}, path string, errs *[]error) []interface{} {
	for idx, patternElement := range patternList {
		curPath := path + "/" + strconv.Itoa(idx)
		value := subVars(ctx, patternElement, curPath, errs)
		patternList[idx] = value
	}
	return patternList
}

func subVal(ctx context.EvalInterface, valuePattern string, path string, errs *[]error) interface{} {
	operatorVariable := getOp(valuePattern)
	variable := valuePattern[len(operatorVariable):]
	// substitute variable with value
	value, failedVars := getValQuery(ctx, variable)
	// if there are failedVars at this level
	// capture as error and the path to the variables
	for _, failedVar := range failedVars {
		failedPath := path + "/" + failedVar
		*errs = append(*errs, NewInvalidPath(failedPath))
	}
	if operatorVariable == "" {
		// default or operator.Equal
		// equal + string value
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

func getOp(pattern string) string {
	operatorVariable := operator.GetOperatorFromStringPattern(pattern)
	if operatorVariable == operator.Equal {
		return ""
	}
	return string(operatorVariable)
}

func getValQuery(ctx context.EvalInterface, valuePattern string) (interface{}, []string) {
	var emptyInterface interface{}
	validRegex := regexp.MustCompile(variableRegex)
	groups := validRegex.FindAllStringSubmatch(valuePattern, -1)
	// there can be multiple varialbes in a single value pattern
	varMap, failedVars := getVal(ctx, groups)
	if len(varMap) == 0 && len(failedVars) == 0 {
		// no variables
		// return original value
		return valuePattern, nil
	}
	if isAllStrings(varMap) {
		newVal := valuePattern
		for key, value := range varMap {
			if val, ok := value.(string); ok {
				newVal = strings.Replace(newVal, key, val, -1)
			}
		}
		return newVal, failedVars
	}
	// multiple substitution per statement for non-string types are not supported
	for _, value := range varMap {
		return value, failedVars
	}
	return emptyInterface, failedVars
}

func getVal(ctx context.EvalInterface, groups [][]string) (map[string]interface{}, []string) {
	substiutions := map[string]interface{}{}
	var failedVars []string
	for _, group := range groups {
		// 0th is the string
		varName := group[0]
		varValue := group[1]
		variable, err := ctx.Query(varValue)
		// err !=nil -> invalid expression
		// err == nil && variable == nil -> variable is empty or path is not present
		// a variable with empty value is considered as a failed variable
		if err != nil || (err == nil && variable == nil) {
			// could not find the variable at the given path
			failedVars = append(failedVars, varName)
			continue
		}
		substiutions[varName] = variable
	}
	return substiutions, failedVars
}

func isAllStrings(subVar map[string]interface{}) bool {
	if len(subVar) == 0 {
		return false
	}
	for _, value := range subVar {
		if _, ok := value.(string); !ok {
			return false
		}
	}
	return true
}

//InvalidPath stores the path to failed variable
type InvalidPath struct {
	path string
}

func (e *InvalidPath) Error() string {
	return e.path
}

//NewInvalidPath returns a new Invalid Path error
func NewInvalidPath(path string) *InvalidPath {
	return &InvalidPath{path: path}
}
