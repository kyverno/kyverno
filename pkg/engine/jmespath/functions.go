package jmespath

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	trunc "github.com/aquilax/truncate"
	"github.com/blang/semver/v4"
	gojmespath "github.com/jmespath/go-jmespath"
	wildcard "github.com/kyverno/go-wildcard"
	"sigs.k8s.io/yaml"
)

var (
	JpObject      = gojmespath.JpObject
	JpString      = gojmespath.JpString
	JpNumber      = gojmespath.JpNumber
	JpArray       = gojmespath.JpArray
	JpArrayString = gojmespath.JpArrayString
	JpAny         = gojmespath.JpAny
	JpBool        = gojmespath.JpType("bool")
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
	patternMatch           = "pattern_match"
	labelMatch             = "label_match"
	add                    = "add"
	subtract               = "subtract"
	multiply               = "multiply"
	divide                 = "divide"
	modulo                 = "modulo"
	base64Decode           = "base64_decode"
	base64Encode           = "base64_encode"
	timeSince              = "time_since"
	pathCanonicalize       = "path_canonicalize"
	truncate               = "truncate"
	semverCompare          = "semver_compare"
	parseJson              = "parse_json"
	parseYAML              = "parse_yaml"
	items                  = "items"
	objectFromLists        = "object_from_lists"
	isExternalURL          = "is_external_url"
)

const errorPrefix = "JMESPath function '%s': "
const invalidArgumentTypeError = errorPrefix + "%d argument is expected of %s type"
const genericError = errorPrefix + "%s"
const zeroDivisionError = errorPrefix + "Zero divisor passed"
const undefinedQuoError = errorPrefix + "Undefined quotient"
const nonIntModuloError = errorPrefix + "Non-integer argument(s) passed for modulo"

type FunctionEntry struct {
	Entry      *gojmespath.FunctionEntry
	Note       string
	ReturnType []JpType
}

func (f *FunctionEntry) String() string {
	args := []string{}
	for _, a := range f.Entry.Arguments {
		aTypes := []string{}
		for _, t := range a.Types {
			aTypes = append(aTypes, string(t))
		}
		args = append(args, strings.Join(aTypes, "|"))
	}
	returnArgs := []string{}
	for _, ra := range f.ReturnType {
		returnArgs = append(returnArgs, string(ra))
	}
	output := fmt.Sprintf("%s(%s) %s", f.Entry.Name, strings.Join(args, ", "), strings.Join(returnArgs, ","))
	if f.Note != "" {
		output += fmt.Sprintf(" (%s)", f.Note)
	}
	return output
}

func GetFunctions() []*FunctionEntry {
	return []*FunctionEntry{
		{
			Entry: &gojmespath.FunctionEntry{Name: compare,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
					{Types: []JpType{JpString}},
				},
				Handler: jpfCompare,
			},
			ReturnType: []JpType{JpBool},
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: equalFold,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
					{Types: []JpType{JpString}},
				},
				Handler: jpfEqualFold,
			},
			ReturnType: []JpType{JpBool},
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: replace,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
					{Types: []JpType{JpString}},
					{Types: []JpType{JpString}},
					{Types: []JpType{JpNumber}},
				},
				Handler: jpfReplace,
			},
			ReturnType: []JpType{JpString},
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: replaceAll,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
					{Types: []JpType{JpString}},
					{Types: []JpType{JpString}},
				},
				Handler: jpfReplaceAll,
			},
			ReturnType: []JpType{JpString},
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: toUpper,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
				},
				Handler: jpfToUpper,
			},
			ReturnType: []JpType{JpString},
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: toLower,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
				},
				Handler: jpfToLower,
			},
			ReturnType: []JpType{JpString},
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: trim,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
					{Types: []JpType{JpString}},
				},
				Handler: jpfTrim,
			},
			ReturnType: []JpType{JpString},
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: split,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
					{Types: []JpType{JpString}},
				},
				Handler: jpfSplit,
			},
			ReturnType: []JpType{JpArrayString},
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: regexReplaceAll,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
					{Types: []JpType{JpString, JpNumber}},
					{Types: []JpType{JpString, JpNumber}},
				},
				Handler: jpRegexReplaceAll,
			},
			ReturnType: []JpType{JpString},
			Note:       "converts all parameters to string",
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: regexReplaceAllLiteral,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
					{Types: []JpType{JpString, JpNumber}},
					{Types: []JpType{JpString, JpNumber}},
				},
				Handler: jpRegexReplaceAllLiteral,
			},
			ReturnType: []JpType{JpString},
			Note:       "converts all parameters to string",
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: regexMatch,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
					{Types: []JpType{JpString, JpNumber}},
				},
				Handler: jpRegexMatch,
			},
			ReturnType: []JpType{JpBool},
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: patternMatch,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
					{Types: []JpType{JpString, JpNumber}},
				},
				Handler: jpPatternMatch,
			},
			ReturnType: []JpType{JpBool},
			Note:       "'*' matches zero or more alphanumeric characters, '?' matches a single alphanumeric character",
		},
		{
			// Validates if label (param1) would match pod/host/etc labels (param2)
			Entry: &gojmespath.FunctionEntry{Name: labelMatch,
				Arguments: []ArgSpec{
					{Types: []JpType{JpObject}},
					{Types: []JpType{JpObject}},
				},
				Handler: jpLabelMatch,
			},
			ReturnType: []JpType{JpBool},
			Note:       "object arguments must be enclosed in backticks; ex. `{{request.object.spec.template.metadata.labels}}`",
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: add,
				Arguments: []ArgSpec{
					{Types: []JpType{JpAny}},
					{Types: []JpType{JpAny}},
				},
				Handler: jpAdd,
			},
			ReturnType: []JpType{JpAny},
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: subtract,
				Arguments: []ArgSpec{
					{Types: []JpType{JpAny}},
					{Types: []JpType{JpAny}},
				},
				Handler: jpSubtract,
			},
			ReturnType: []JpType{JpAny},
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: multiply,
				Arguments: []ArgSpec{
					{Types: []JpType{JpAny}},
					{Types: []JpType{JpAny}},
				},
				Handler: jpMultiply,
			},
			ReturnType: []JpType{JpAny},
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: divide,
				Arguments: []ArgSpec{
					{Types: []JpType{JpAny}},
					{Types: []JpType{JpAny}},
				},
				Handler: jpDivide,
			},
			ReturnType: []JpType{JpAny},
			Note:       "divisor must be non zero",
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: modulo,
				Arguments: []ArgSpec{
					{Types: []JpType{JpAny}},
					{Types: []JpType{JpAny}},
				},
				Handler: jpModulo,
			},
			ReturnType: []JpType{JpAny},
			Note:       "divisor must be non-zero, arguments must be integers",
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: base64Decode,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
				},
				Handler: jpBase64Decode,
			},
			ReturnType: []JpType{JpString},
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: base64Encode,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
				},
				Handler: jpBase64Encode,
			},
			ReturnType: []JpType{JpString},
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: timeSince,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
					{Types: []JpType{JpString}},
					{Types: []JpType{JpString}},
				},
				Handler: jpTimeSince,
			},
			ReturnType: []JpType{JpString},
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: pathCanonicalize,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
				},
				Handler: jpPathCanonicalize,
			},
			ReturnType: []JpType{JpString},
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: truncate,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
					{Types: []JpType{JpNumber}},
				},
				Handler: jpTruncate,
			},
			ReturnType: []JpType{JpString},
			Note:       "length argument must be enclosed in backticks; ex. \"{{request.object.metadata.name | truncate(@, `9`)}}\"",
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: semverCompare,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
					{Types: []JpType{JpString}},
				},
				Handler: jpSemverCompare,
			},
			ReturnType: []JpType{JpBool},
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: parseJson,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
				},
				Handler: jpParseJson,
			},
			ReturnType: []JpType{JpAny},
			Note:       "decodes a valid JSON encoded string to the appropriate type. Opposite of `to_string` function",
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: parseYAML,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
				},
				Handler: jpParseYAML,
			},
			ReturnType: []JpType{JpAny},
			Note:       "decodes a valid YAML encoded string to the appropriate type provided it can be represented as JSON",
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: items,
				Arguments: []ArgSpec{
					{Types: []JpType{JpObject}},
					{Types: []JpType{JpString}},
					{Types: []JpType{JpString}},
				},
				Handler: jpItems,
			},
			ReturnType: []JpType{JpArray},
			Note:       "converts a map to an array of objects where each key:value is an item in the array",
		},
		{
			Entry: &gojmespath.FunctionEntry{Name: objectFromLists,
				Arguments: []ArgSpec{
					{Types: []JpType{JpArray}},
					{Types: []JpType{JpArray}},
				},
				Handler: jpObjectFromLists,
			},
			ReturnType: []JpType{JpObject},
			Note:       "converts a pair of lists containing keys and values to an object",
		}, {
			Entry: &gojmespath.FunctionEntry{
				Name: isExternalURL,
				Arguments: []ArgSpec{
					{Types: []JpType{JpString}},
				},
				Handler: jpIsExternalURL,
			},
			ReturnType: []JpType{JpBool},
			Note:       "determine if a URL points to an external network address",
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

func jpPatternMatch(arguments []interface{}) (interface{}, error) {
	pattern, err := validateArg(regexMatch, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}

	src, err := ifaceToString(arguments[1])
	if err != nil {
		return nil, fmt.Errorf(invalidArgumentTypeError, regexMatch, 2, "String or Real")
	}

	return wildcard.Match(pattern.String(), src), nil
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
	op1, op2, err := ParseArithemticOperands(arguments, add)
	if err != nil {
		return nil, err
	}

	return op1.Add(op2)
}

func jpSubtract(arguments []interface{}) (interface{}, error) {
	op1, op2, err := ParseArithemticOperands(arguments, subtract)
	if err != nil {
		return nil, err
	}

	return op1.Subtract(op2)
}

func jpMultiply(arguments []interface{}) (interface{}, error) {
	op1, op2, err := ParseArithemticOperands(arguments, multiply)
	if err != nil {
		return nil, err
	}

	return op1.Multiply(op2)
}

func jpDivide(arguments []interface{}) (interface{}, error) {
	op1, op2, err := ParseArithemticOperands(arguments, divide)
	if err != nil {
		return nil, err
	}

	return op1.Divide(op2)
}

func jpModulo(arguments []interface{}) (interface{}, error) {
	op1, op2, err := ParseArithemticOperands(arguments, modulo)
	if err != nil {
		return nil, err
	}

	return op1.Modulo(op2)
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

func jpTimeSince(arguments []interface{}) (interface{}, error) {
	var err error
	layout, err := validateArg("", arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}

	ts1, err := validateArg("", arguments, 1, reflect.String)
	if err != nil {
		return nil, err
	}

	ts2, err := validateArg("", arguments, 2, reflect.String)
	if err != nil {
		return nil, err
	}

	var t1, t2 time.Time
	if layout.String() != "" {
		t1, err = time.Parse(layout.String(), ts1.String())
	} else {
		t1, err = time.Parse(time.RFC3339, ts1.String())
	}
	if err != nil {
		return nil, err
	}

	t2 = time.Now()
	if ts2.String() != "" {
		if layout.String() != "" {
			t2, err = time.Parse(layout.String(), ts2.String())
		} else {
			t2, err = time.Parse(time.RFC3339, ts2.String())
		}

		if err != nil {
			return nil, err
		}
	}

	return t2.Sub(t1).String(), nil
}

func jpPathCanonicalize(arguments []interface{}) (interface{}, error) {
	var err error
	str, err := validateArg(pathCanonicalize, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}

	return filepath.Join(str.String()), nil
}

func jpTruncate(arguments []interface{}) (interface{}, error) {
	var err error
	var normalizedLength float64
	str, err := validateArg(truncate, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}
	length, err := validateArg(truncate, arguments, 1, reflect.Float64)
	if err != nil {
		return nil, err
	}

	if length.Float() < 0 {
		normalizedLength = float64(0)
	} else {
		normalizedLength = length.Float()
	}

	return trunc.Truncator(str.String(), int(normalizedLength), trunc.CutStrategy{}), nil
}

func jpSemverCompare(arguments []interface{}) (interface{}, error) {
	var err error
	v, err := validateArg(semverCompare, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}

	r, err := validateArg(semverCompare, arguments, 1, reflect.String)
	if err != nil {
		return nil, err
	}

	version, _ := semver.Parse(v.String())
	expectedRange, err := semver.ParseRange(r.String())
	if err != nil {
		return nil, err
	}

	if expectedRange(version) {
		return true, nil
	}
	return false, nil
}

func jpParseJson(arguments []interface{}) (interface{}, error) {
	input, err := validateArg(parseJson, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}
	var output interface{}
	err = json.Unmarshal([]byte(input.String()), &output)
	return output, err
}

func jpParseYAML(arguments []interface{}) (interface{}, error) {
	input, err := validateArg(parseYAML, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}
	jsonData, err := yaml.YAMLToJSON([]byte(input.String()))
	if err != nil {
		return nil, err
	}
	var output interface{}
	err = json.Unmarshal(jsonData, &output)
	return output, err
}

func jpItems(arguments []interface{}) (interface{}, error) {
	input, ok := arguments[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf(invalidArgumentTypeError, arguments, 0, "Object")
	}
	keyName, ok := arguments[1].(string)
	if !ok {
		return nil, fmt.Errorf(invalidArgumentTypeError, arguments, 1, "String")
	}
	valName, ok := arguments[2].(string)
	if !ok {
		return nil, fmt.Errorf(invalidArgumentTypeError, arguments, 2, "String")
	}

	arrayOfObj := make([]map[string]interface{}, 0)

	keys := []string{}

	// Sort the keys so that the output is deterministic
	for key := range input {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		m := make(map[string]interface{})
		m[keyName] = key
		m[valName] = input[key]
		arrayOfObj = append(arrayOfObj, m)
	}

	return arrayOfObj, nil
}

func jpObjectFromLists(arguments []interface{}) (interface{}, error) {
	keys, ok := arguments[0].([]interface{})
	if !ok {
		return nil, fmt.Errorf(invalidArgumentTypeError, arguments, 0, "Array")
	}
	values, ok := arguments[1].([]interface{})
	if !ok {
		return nil, fmt.Errorf(invalidArgumentTypeError, arguments, 1, "Array")
	}

	output := map[string]interface{}{}

	for i, ikey := range keys {
		key, err := ifaceToString(ikey)
		if err != nil {
			return nil, fmt.Errorf(invalidArgumentTypeError, arguments, 0, "StringArray")
		}
		if i < len(values) {
			output[key] = values[i]
		} else {
			output[key] = nil
		}
	}

	return output, nil
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
func jpIsExternalURL(arguments []interface{}) (interface{}, error) {
	var err error
	str, err := validateArg(pathCanonicalize, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}
	parsedURL, err := url.Parse(str.String())
	if err != nil {
		return false, err
	}
	ip := net.ParseIP(parsedURL.Hostname())
	if ip != nil {
		return !(ip.IsLoopback() || ip.IsPrivate()), nil
	}
	// If it can't be parsed as an IP, then resolve the domain name
	ips, err := net.LookupIP(parsedURL.Hostname())
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() {
			return false, nil
		}
	}
	return true, nil
}
func validateArg(f string, arguments []interface{}, index int, expectedType reflect.Kind) (reflect.Value, error) {
	arg := reflect.ValueOf(arguments[index])
	if arg.Type().Kind() != expectedType {
		return reflect.Value{}, fmt.Errorf(invalidArgumentTypeError, f, index+1, expectedType.String())
	}

	return arg, nil
}
