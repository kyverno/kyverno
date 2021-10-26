package jmespath

import (
	"encoding/base64"
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
	equalFold              = "equal_fold"
	replace                = "replace"
	replaceAll             = "replace_all"
	toUpper                = "to_upper"
	toLower                = "to_lower"
	trim                   = "trim"
	split                  = "split"
	regexReplaceAll        = "regex_replace_all"
	regexReplaceAllLiteral = "regex_replace_all_literal"
	regexMatch             = "regex_match"
	labelMatch             = "label_match"
	add                    = "add"
	subtract               = "subtract"
	multiply               = "multiply"
	divide                 = "divide"
	modulo                 = "modulo"
	base64Decode           = "base64_decode"
	base64Encode           = "base64_encode"
)

const errorPrefix = "JMESPath function '%s': "
const invalidArgumentTypeError = errorPrefix + "%d argument is expected of %s type"
const genericError = errorPrefix + "%s"
const zeroDivisionError = errorPrefix + "Zero divisor passed"
const nonIntModuloError = errorPrefix + "Non-integer argument(s) passed for modulo"

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
		{
			Name: add,
			Arguments: []ArgSpec{
				{Types: []JpType{JpNumber}},
				{Types: []JpType{JpNumber}},
			},
			Handler: jpAdd,
		},
		{
			Name: subtract,
			Arguments: []ArgSpec{
				{Types: []JpType{JpNumber}},
				{Types: []JpType{JpNumber}},
			},
			Handler: jpSubtract,
		},
		{
			Name: multiply,
			Arguments: []ArgSpec{
				{Types: []JpType{JpNumber}},
				{Types: []JpType{JpNumber}},
			},
			Handler: jpMultiply,
		},
		{
			Name: divide,
			Arguments: []ArgSpec{
				{Types: []JpType{JpNumber}},
				{Types: []JpType{JpNumber}},
			},
			Handler: jpDivide,
		},
		{
			Name: modulo,
			Arguments: []ArgSpec{
				{Types: []JpType{JpNumber}},
				{Types: []JpType{JpNumber}},
			},
			Handler: jpModulo,
		},
		{
			Name: base64Decode,
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
			},
			Handler: jpBase64Decode,
		},
		{
			Name: base64Encode,
			Arguments: []ArgSpec{
				{Types: []JpType{JpString}},
			},
			Handler: jpBase64Encode,
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

	split := strings.Split(str.String(), sep.String())
	arr := make([]interface{}, len(split))

	for i, v := range split {
		arr[i] = v
	}

	return arr, nil
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

func jpAdd(arguments []interface{}) (interface{}, error) {
	var err error
	op1, err := validateArg(divide, arguments, 0, reflect.Float64)
	if err != nil {
		return nil, err
	}

	op2, err := validateArg(divide, arguments, 1, reflect.Float64)
	if err != nil {
		return nil, err
	}

	return op1.Float() + op2.Float(), nil
}

func jpSubtract(arguments []interface{}) (interface{}, error) {
	var err error
	op1, err := validateArg(divide, arguments, 0, reflect.Float64)
	if err != nil {
		return nil, err
	}

	op2, err := validateArg(divide, arguments, 1, reflect.Float64)
	if err != nil {
		return nil, err
	}

	return op1.Float() - op2.Float(), nil
}

func jpMultiply(arguments []interface{}) (interface{}, error) {
	var err error
	op1, err := validateArg(divide, arguments, 0, reflect.Float64)
	if err != nil {
		return nil, err
	}

	op2, err := validateArg(divide, arguments, 1, reflect.Float64)
	if err != nil {
		return nil, err
	}

	return op1.Float() * op2.Float(), nil
}

func jpDivide(arguments []interface{}) (interface{}, error) {
	var err error
	op1, err := validateArg(divide, arguments, 0, reflect.Float64)
	if err != nil {
		return nil, err
	}

	op2, err := validateArg(divide, arguments, 1, reflect.Float64)
	if err != nil {
		return nil, err
	}

	if op2.Float() == 0 {
		return nil, fmt.Errorf(zeroDivisionError, divide)
	}

	return op1.Float() / op2.Float(), nil
}

func jpModulo(arguments []interface{}) (interface{}, error) {
	var err error
	op1, err := validateArg(divide, arguments, 0, reflect.Float64)
	if err != nil {
		return nil, err
	}

	op2, err := validateArg(divide, arguments, 1, reflect.Float64)
	if err != nil {
		return nil, err
	}

	val1 := int64(op1.Float())
	val2 := int64(op2.Float())

	if op1.Float() != float64(val1) {
		return nil, fmt.Errorf(nonIntModuloError, modulo)
	}

	if op2.Float() != float64(val2) {
		return nil, fmt.Errorf(nonIntModuloError, modulo)
	}

	if val2 == 0 {
		return nil, fmt.Errorf(zeroDivisionError, modulo)
	}

	return val1 % val2, nil
}

func jpBase64Decode(arguments []interface{}) (interface{}, error) {
	var err error
	str, err := validateArg("", arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}

	decodedStr, err := base64.StdEncoding.DecodeString(str.String())
	if err != nil {
		return nil, err
	}

	return string(decodedStr), nil
}

func jpBase64Encode(arguments []interface{}) (interface{}, error) {
	var err error
	str, err := validateArg("", arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}

	return base64.StdEncoding.EncodeToString([]byte(str.String())), nil
}

// InterfaceToString casts an interface to a string type
func ifaceToString(iface interface{}) (string, error) {
	switch i := iface.(type) {
	case int:
		return strconv.Itoa(i), nil
	case float64:
		return strconv.FormatFloat(i, 'f', -1, 32), nil
	case float32:
		return strconv.FormatFloat(float64(i), 'f', -1, 32), nil
	case string:
		return i, nil
	case bool:
		return strconv.FormatBool(i), nil
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
