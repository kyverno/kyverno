package jmespath

import (
	"reflect"
)

func validateArg(f string, arguments []interface{}, index int, expectedType reflect.Kind) (reflect.Value, error) {
	if index >= len(arguments) {
		return reflect.Value{}, formatError(argOutOfBoundsError, f, index+1, len(arguments))
	}
	if arguments[index] == nil {
		return reflect.Value{}, formatError(invalidArgumentTypeError, f, index+1, expectedType.String())
	}
	arg := reflect.ValueOf(arguments[index])
	if arg.Type().Kind() != expectedType {
		return reflect.Value{}, formatError(invalidArgumentTypeError, f, index+1, expectedType.String())
	}
	return arg, nil
}
