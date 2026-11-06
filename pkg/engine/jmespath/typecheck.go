package jmespath

import (
	"reflect"
	"strings"

	gojmespath "github.com/kyverno/go-jmespath"
)

// registrable returns the entry that is handed over to the JMESPath function
// caller. The declared argument types are replaced by `any` so that the library
// does not reject the call before the handler runs, and the handler is wrapped
// with an equivalent check that reports the offending function and argument.
//
// Without this, a mistyped argument surfaces as the library's raw type error,
// which mentions neither the function nor the argument index.
func (f FunctionEntry) registrable() gojmespath.FunctionEntry {
	entry := f.FunctionEntry
	specs := entry.Arguments
	if len(specs) == 0 {
		return entry
	}
	anySpecs := make([]argSpec, len(specs))
	for i := range anySpecs {
		anySpecs[i] = argSpec{Types: []jpType{jpAny}}
	}
	handler := entry.Handler
	name := entry.Name
	entry.Arguments = anySpecs
	entry.Handler = func(arguments []any) (any, error) {
		// expression references are prepended to the arguments by the caller
		args := arguments
		if entry.HasExpRef && len(args) >= 2 {
			args = args[2:]
		}
		for i, spec := range specs {
			if i >= len(args) {
				break
			}
			if !matchesAnyType(spec.Types, args[i]) {
				return nil, formatError(argumentTypeMismatchError, name, i+1, joinTypes(spec.Types), typeName(args[i]))
			}
		}
		return handler(arguments)
	}
	return entry
}

// matchesAnyType mirrors the type checking performed by the JMESPath library.
// Types it doesn't know about are accepted so that the check never rejects an
// argument the library would have allowed.
func matchesAnyType(types []jpType, arg any) bool {
	for _, t := range types {
		switch t {
		case jpAny:
			return true
		case jpString:
			if _, ok := arg.(string); ok {
				return true
			}
		case jpNumber:
			if _, ok := arg.(float64); ok {
				return true
			}
		case jpObject:
			if _, ok := arg.(map[string]any); ok {
				return true
			}
		case jpArray:
			if isSlice(arg) {
				return true
			}
		default:
			return true
		}
	}
	return false
}

func isSlice(arg any) bool {
	if arg == nil {
		return false
	}
	return reflect.TypeOf(arg).Kind() == reflect.Slice
}

func joinTypes(types []jpType) string {
	names := make([]string, 0, len(types))
	for _, t := range types {
		names = append(names, string(t))
	}
	return strings.Join(names, " or ")
}

// typeName reports the JMESPath type of a value, for use in error messages.
func typeName(arg any) string {
	switch value := arg.(type) {
	case nil:
		return "null"
	case string:
		return string(jpString)
	case float64:
		return string(jpNumber)
	case bool:
		return string(jpBool)
	case map[string]any:
		return string(jpObject)
	default:
		if isSlice(value) {
			return string(jpArray)
		}
		return reflect.TypeOf(arg).String()
	}
}
