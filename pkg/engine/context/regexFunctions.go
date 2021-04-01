package context

import (
	"errors"
	"regexp"
	"strconv"

	jp "github.com/jmespath/go-jmespath"
)

// GetRegexFunctions returns a slice from *jmespath.FunctionEntry
// that contains the prototypes of the regular expression functions
func GetRegexFunctions() []*jp.FunctionEntry {
	return []*jp.FunctionEntry{
		getRegexMatch(),
		getRegexReplaceAll(),
		getRegexReplaceAllLiteral(),
	}
}

func getRegexReplaceAll() *jp.FunctionEntry {
	return &jp.FunctionEntry{
		Name: "regexReplaceAll",
		Arguments: []jp.ArgSpec{
			{Types: []jp.JpType{jp.JpString}},
			{Types: []jp.JpType{jp.JpString, jp.JpNumber}},
			{Types: []jp.JpType{jp.JpString, jp.JpNumber}},
		},
		Handler: func(arguments []interface{}) (interface{}, error) {
			regex := arguments[0].(string)
			src, err := toString(arguments[1])
			if err != nil {
				return nil, err
			}
			repl, err := toString(arguments[2])
			if err != nil {
				return nil, err
			}
			reg, err := regexp.Compile(regex)
			if err != nil {
				return nil, err
			}
			return reg.ReplaceAll([]byte(src), []byte(repl)), nil
		},
	}
}

func getRegexReplaceAllLiteral() *jp.FunctionEntry {
	return &jp.FunctionEntry{
		Name: "regexReplaceAllLiteral",
		Arguments: []jp.ArgSpec{
			{Types: []jp.JpType{jp.JpString}},
			{Types: []jp.JpType{jp.JpString, jp.JpNumber}},
			{Types: []jp.JpType{jp.JpString, jp.JpNumber}},
		},
		Handler: func(arguments []interface{}) (interface{}, error) {
			regex := arguments[0].(string)
			src, err := toString(arguments[1])
			if err != nil {
				return nil, err
			}
			repl, err := toString(arguments[2])
			if err != nil {
				return nil, err
			}
			reg, err := regexp.Compile(regex)
			if err != nil {
				return nil, err
			}
			return reg.ReplaceAllLiteral([]byte(src), []byte(repl)), nil
		},
	}
}

func getRegexMatch() *jp.FunctionEntry {
	return &jp.FunctionEntry{
		Name: "regexMatch",
		Arguments: []jp.ArgSpec{
			{Types: []jp.JpType{jp.JpString}},
			{Types: []jp.JpType{jp.JpString, jp.JpNumber}},
		},
		Handler: func(arguments []interface{}) (interface{}, error) {
			regex := arguments[0].(string)
			src, err := toString(arguments[1])
			if err != nil {
				return nil, err
			}
			return regexp.Match(regex, []byte(src))
		},
	}
}

func toString(iface interface{}) (string, error) {
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
