package jmespath

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	gojmespath "github.com/jmespath/go-jmespath"
)

var (
	JpObject      = gojmespath.JpObject
	JpString      = gojmespath.JpString
	JpNumber      = gojmespath.JpNumber
	JpArray       = gojmespath.JpArray
	JpArrayString = gojmespath.JpArrayString
)

type (
	JpType  = gojmespath.JpType
	ArgSpec = gojmespath.ArgSpec
)

// function names
var (
	compare                = "compare"
	contains               = "contains"
	equalFold              = "equal_fold"
	replace                = "replace"
	replaceAll             = "replace_all"
	toUpper                = "to_upper"
	toLower                = "to_lower"
	trim                   = "trim"
	split                  = "split"
	equals                 = "equals"
	regexReplaceAll        = "regex_replace_all"
	regexReplaceAllLiteral = "regex_replace_all_literal"
	regexMatch             = "regex_match"
	labelMatch             = "label_match"
)

const errorPrefix = "JMESPath function '%s': "
const invalidArgumentTypeError = errorPrefix + "%d argument is expected of %s type"
const genericError = errorPrefix + "%s"

func getFunctions() []*gojmespath.FunctionEntry {
	return []*gojmespath.FunctionEntry{
		{
			Name: compare,
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
			},
			Handler: jpfCompare,
		},
		{
			Name: contains,
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
			},
			Handler: jpfContains,
		},
		{
			Name: equalFold,
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
			},
			Handler: jpfEqualFold,
		},
		{
			Name: replace,
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
				{Types: []JpType{JpNumber}},
			},
			Handler: jpfReplace,
		},
		{
			Name: replaceAll,
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
			},
			Handler: jpfReplaceAll,
		},
		{
			Name: toUpper,
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
			},
			Handler: jpfToUpper,
		},
		{
			Name: toLower,
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
			},
			Handler: jpfToLower,
		},
		{
			Name: trim,
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
			},
			Handler: jpfTrim,
		},
		{
			Name: split,
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
			},
			Handler: jpfSplit,
		},
		{
			Name: regexReplaceAll,
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString, JpNumber}},
				{Types: []JpType{JpString, JpNumber}},
			},
			Handler: jpRegexReplaceAll,
		},
		{
			Name: regexReplaceAllLiteral,
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString, JpNumber}},
				{Types: []JpType{JpString, JpNumber}},
			},
			Handler: jpRegexReplaceAllLiteral,
		},
		{
			Name: regexMatch,
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString, JpNumber}},
			},
			Handler: jpRegexMatch,
		},
		{
			// Validates if label (param1) would match pod/host/etc labels (param2)
			Name: labelMatch,
			Arguments: []ArgSpec{
				{Types: []JpType{JpObject}},
				{Types: []JpType{JpObject}},
			},
			Handler: jpLabelMatch,
		},
	}

}

func jpfCompare(arguments []interface{}) (interface{}, error) {
	var err error
	a, err := validateArg(compare, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}

	b, err := validateArg(compare, arguments, 1, reflect.String)
	if err != nil {
		return nil, err
	}

	return strings.Compare(a.String(), b.String()), nil
}

func jpfContains(arguments []interface{}) (interface{}, error) {
	var err error
	str, err := validateArg(contains, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}

	substr, err := validateArg(contains, arguments, 1, reflect.String)
	if err != nil {
		return nil, err
	}

	return strings.Contains(str.String(), substr.String()), nil
}

func jpfEqualFold(arguments []interface{}) (interface{}, error) {
	var err error
	a, err := validateArg(equalFold, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}

	b, err := validateArg(equalFold, arguments, 1, reflect.String)
	if err != nil {
		return nil, err
	}

	return strings.EqualFold(a.String(), b.String()), nil
}

func jpfReplace(arguments []interface{}) (interface{}, error) {
	var err error
	str, err := validateArg(replace, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}

	old, err := validateArg(replace, arguments, 1, reflect.String)
	if err != nil {
		return nil, err
	}

	new, err := validateArg(replace, arguments, 2, reflect.String)
	if err != nil {
		return nil, err
	}

	n, err := validateArg(replace, arguments, 3, reflect.Float64)
	if err != nil {
		return nil, err
	}

	return strings.Replace(str.String(), old.String(), new.String(), int(n.Float())), nil
}

func jpfReplaceAll(arguments []interface{}) (interface{}, error) {
	var err error
	str, err := validateArg(replaceAll, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}

	old, err := validateArg(replaceAll, arguments, 1, reflect.String)
	if err != nil {
		return nil, err
	}

	new, err := validateArg(replaceAll, arguments, 2, reflect.String)
	if err != nil {
		return nil, err
	}

	return strings.ReplaceAll(str.String(), old.String(), new.String()), nil
}

func jpfToUpper(arguments []interface{}) (interface{}, error) {
	var err error
	str, err := validateArg(toUpper, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}

	return strings.ToUpper(str.String()), nil
}

func jpfToLower(arguments []interface{}) (interface{}, error) {
	var err error
	str, err := validateArg(toLower, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}

	return strings.ToLower(str.String()), nil
}

func jpfTrim(arguments []interface{}) (interface{}, error) {
	var err error
	str, err := validateArg(trim, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}

	cutset, err := validateArg(trim, arguments, 1, reflect.String)
	if err != nil {
		return nil, err
	}

	return strings.Trim(str.String(), cutset.String()), nil
}

func jpfSplit(arguments []interface{}) (interface{}, error) {
	var err error
	str, err := validateArg(split, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}

	sep, err := validateArg(split, arguments, 1, reflect.String)
	if err != nil {
		return nil, err
	}

	return strings.Split(str.String(), sep.String()), nil
}

func jpRegexReplaceAll(arguments []interface{}) (interface{}, error) {
	var err error
	regex, err := validateArg(regexReplaceAll, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}

	src, err := ifaceToString(arguments[1])
	if err != nil {
		return nil, fmt.Errorf(invalidArgumentTypeError, regexReplaceAll, 2, "String or Real")
	}

	repl, err := ifaceToString(arguments[2])
	if err != nil {
		return nil, fmt.Errorf(invalidArgumentTypeError, regexReplaceAll, 3, "String or Real")
	}

	reg, err := regexp.Compile(regex.String())
	if err != nil {
		return nil, fmt.Errorf(genericError, regexReplaceAll, err.Error())
	}
	return string(reg.ReplaceAll([]byte(src), []byte(repl))), nil
}

func jpRegexReplaceAllLiteral(arguments []interface{}) (interface{}, error) {
	var err error
	regex, err := validateArg(regexReplaceAllLiteral, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}

	src, err := ifaceToString(arguments[1])
	if err != nil {
		return nil, fmt.Errorf(invalidArgumentTypeError, regexReplaceAllLiteral, 2, "String or Real")
	}

	repl, err := ifaceToString(arguments[2])
	if err != nil {
		return nil, fmt.Errorf(invalidArgumentTypeError, regexReplaceAllLiteral, 3, "String or Real")
	}

	reg, err := regexp.Compile(regex.String())
	if err != nil {
		return nil, fmt.Errorf(genericError, regexReplaceAllLiteral, err.Error())
	}
	return string(reg.ReplaceAllLiteral([]byte(src), []byte(repl))), nil
}

func jpRegexMatch(arguments []interface{}) (interface{}, error) {
	var err error
	regex, err := validateArg(regexMatch, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}

	src, err := ifaceToString(arguments[1])
	if err != nil {
		return nil, fmt.Errorf(invalidArgumentTypeError, regexMatch, 2, "String or Real")
	}

	return regexp.Match(regex.String(), []byte(src))
}

func jpLabelMatch(arguments []interface{}) (interface{}, error) {
	labelMap, ok := arguments[0].(map[string]interface{})

	if !ok {
		return nil, fmt.Errorf(invalidArgumentTypeError, labelMatch, 0, "Object")
	}

	matchMap, ok := arguments[1].(map[string]interface{})

	if !ok {
		return nil, fmt.Errorf(invalidArgumentTypeError, labelMatch, 1, "Object")
	}

	for key, value := range labelMap {
		if val, ok := matchMap[key]; !ok || val != value {
			return false, nil
		}
	}

	return true, nil
}

// InterfaceToString casts an interface to a string type
func ifaceToString(iface interface{}) (string, error) {
	switch iface.(type) {
	case int:
		return strconv.Itoa(iface.(int)), nil
	case float64:
		return strconv.FormatFloat(iface.(float64), 'f', -1, 32), nil
	case float32:
		return strconv.FormatFloat(iface.(float64), 'f', -1, 32), nil
	case string:
		return iface.(string), nil
	case bool:
		return strconv.FormatBool(iface.(bool)), nil
	default:
		return "", errors.New("error, undefined type cast")
	}
}

func validateArg(f string, arguments []interface{}, index int, expectedType reflect.Kind) (reflect.Value, error) {
	arg := reflect.ValueOf(arguments[index])
	if arg.Type().Kind() != expectedType {
		return reflect.Value{}, fmt.Errorf(invalidArgumentTypeError, equalFold, index+1, expectedType.String())
	}

	return arg, nil
}
