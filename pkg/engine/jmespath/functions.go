package jmespath

import (
	"errors"
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

func getFunctions() []*gojmespath.FunctionEntry {
	return []*gojmespath.FunctionEntry{
		{
			Name: "compare",
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
			},
			Handler: jpfCompare,
		},
		{
			Name: "contains",
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
			},
			Handler: jpfContains,
		},
		{
			Name: "equal_fold",
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
			},
			Handler: jpfEqualFold,
		},
		{
			Name: "replace",
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
				{Types: []JpType{JpNumber}},
			},
			Handler: jpfReplace,
		},
		{
			Name: "replace_all",
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
			},
			Handler: jpfReplaceAll,
		},
		{
			Name: "to_upper",
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
			},
			Handler: jpfToUpper,
		},
		{
			Name: "to_lower",
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
			},
			Handler: jpfToLower,
		},
		{
			Name: "trim",
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
			},
			Handler: jpfTrim,
		},
		{
			Name: "split",
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
			},
			Handler: jpfSplit,
		},
		{
			Name: "equals",
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString}},
			},
			Handler: jpEquals,
		},
		{
			Name: "regexReplaceAll",
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString, JpNumber}},
				{Types: []JpType{JpString, JpNumber}},
			},
			Handler: jpRegexReplaceAll,
		},
		{
			Name: "regexReplaceAllLiteral",
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString, JpNumber}},
				{Types: []JpType{JpString, JpNumber}},
			},
			Handler: jpRegexReplaceAllLiteral,
		},
		{
			Name: "regexMatch",
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
				{Types: []JpType{JpString, JpNumber}},
			},
			Handler: jpRegexMatch,
		},
	}

}

func jpfCompare(arguments []interface{}) (interface{}, error) {
	a, ok := arguments[0].(string)
	if !ok {
		return nil, errors.New("Compare: first argument is expected of string type")
	}

	b, ok := arguments[1].(string)
	if !ok {
		return nil, errors.New("Compare: second argument is expected of string type")
	}

	return strings.Compare(a, b), nil
}

func jpfContains(arguments []interface{}) (interface{}, error) {
	str, ok := arguments[0].(string)
	if !ok {
		return nil, errors.New("Contains: first argument is expected of string type")
	}

	substr, ok := arguments[1].(string)
	if !ok {
		return nil, errors.New("Contains: second argument is expected of string type")
	}

	return strings.Contains(str, substr), nil
}

func jpfEqualFold(arguments []interface{}) (interface{}, error) {
	a, ok := arguments[0].(string)
	if !ok {
		return nil, errors.New("EqualFold: first argument is expected of string type")
	}

	b, ok := arguments[1].(string)
	if !ok {
		return nil, errors.New("EqualFold: second argument is expected of string type")
	}

	return strings.EqualFold(a, b), nil
}

func jpfReplace(arguments []interface{}) (interface{}, error) {
	str, ok := arguments[0].(string)
	if !ok {
		return nil, errors.New("Replace: first argument is expected of string type")
	}

	old, ok := arguments[1].(string)
	if !ok {
		return nil, errors.New("Replace: second argument is expected of string type")
	}

	new, ok := arguments[2].(string)
	if !ok {
		return nil, errors.New("Replace: third argument is expected of string type")
	}

	n, ok := arguments[3].(float64)
	if !ok {
		return nil, errors.New("Replace: forth argument is expected of int type")
	}

	return strings.Replace(str, old, new, int(n)), nil
}

func jpfReplaceAll(arguments []interface{}) (interface{}, error) {
	str, ok := arguments[0].(string)
	if !ok {
		return nil, errors.New("ReplaceAll: first argument is expected of string type")
	}

	old, ok := arguments[1].(string)
	if !ok {
		return nil, errors.New("ReplaceAll: second argument is expected of string type")
	}

	new, ok := arguments[2].(string)
	if !ok {
		return nil, errors.New("ReplaceAll: third argument is expected of string type")
	}

	return strings.ReplaceAll(str, old, new), nil
}

func jpfToUpper(arguments []interface{}) (interface{}, error) {
	str, ok := arguments[0].(string)
	if !ok {
		return nil, errors.New("ToUpper: first argument is expected of string type")
	}

	return strings.ToUpper(str), nil
}

func jpfToLower(arguments []interface{}) (interface{}, error) {
	str, ok := arguments[0].(string)
	if !ok {
		return nil, errors.New("ToLower: first argument is expected of string type")
	}

	return strings.ToLower(str), nil
}

func jpfTrim(arguments []interface{}) (interface{}, error) {
	str, ok := arguments[0].(string)
	if !ok {
		return nil, errors.New("Trim: first argument is expected of string type")
	}

	cutset, ok := arguments[1].(string)
	if !ok {
		return nil, errors.New("Trim: second argument is expected of string type")
	}

	return strings.Trim(str, cutset), nil
}

func jpfSplit(arguments []interface{}) (interface{}, error) {
	str, ok := arguments[0].(string)
	if !ok {
		return nil, errors.New("Split: first argument is expected of string type")
	}

	sep, ok := arguments[1].(string)
	if !ok {
		return nil, errors.New("Split: second argument is expected of string type")
	}

	return strings.Split(str, sep), nil
}

func jpEquals(arguments []interface{}) (interface{}, error) {
	first, ok := arguments[0].(string)
	if !ok {
		return nil, errors.New("Equals: first argument is expected of string type")
	}

	second, ok := arguments[1].(string)
	if !ok {
		return nil, errors.New("Equals: second argument is expected of string type")
	}

	return first == second, nil
}

func jpRegexReplaceAll(arguments []interface{}) (interface{}, error) {
	regex, ok := arguments[0].(string)
	if !ok {
		return nil, errors.New("RegexReplaceAll: first argument is expected of string type")
	}
	src, err := ifaceToString(arguments[1])
	if err != nil {
		return nil, err
	}
	repl, err := ifaceToString(arguments[2])
	if err != nil {
		return nil, err
	}
	reg, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}
	return reg.ReplaceAll([]byte(src), []byte(repl)), nil
}

func jpRegexReplaceAllLiteral(arguments []interface{}) (interface{}, error) {
	regex, ok := arguments[0].(string)
	if !ok {
		return nil, errors.New("RegexReplaceAllLiteral: first argument is expected of string type")
	}
	src, err := ifaceToString(arguments[1])
	if err != nil {
		return nil, err
	}
	repl, err := ifaceToString(arguments[2])
	if err != nil {
		return nil, err
	}
	reg, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}
	return reg.ReplaceAllLiteral([]byte(src), []byte(repl)), nil
}

func jpRegexMatch(arguments []interface{}) (interface{}, error) {
	regex, ok := arguments[0].(string)
	if !ok {
		return nil, errors.New("RegexMatch: first argument is expected of string type")
	}
	src, err := ifaceToString(arguments[1])
	if err != nil {
		return nil, err
	}
	return regexp.Match(regex, []byte(src))
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
