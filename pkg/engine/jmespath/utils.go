package jmespath

import (
	"fmt"
	"math"
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

func intNumber(number float64) (int, error) {
	if math.IsInf(number, 0) || math.IsNaN(number) || math.Trunc(number) != number {
		return 0, fmt.Errorf("expected an integer number but got: %g", number)
	}
	intNumber := int(number)
	if float64(intNumber) != number {
		return 0, fmt.Errorf("number is outside the range of integer numbers: %g", number)
	}
	return intNumber, nil
}
