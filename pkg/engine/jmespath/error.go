package jmespath

import (
	"fmt"
)

const (
	errorPrefix              = "JMESPath function '%s': "
	invalidArgumentTypeError = errorPrefix + "%d argument is expected of %s type"
	genericError             = errorPrefix + "%s"
	argOutOfBoundsError      = errorPrefix + "%d argument is out of bounds (%d)"
	zeroDivisionError        = errorPrefix + "Zero divisor passed"
	nonIntModuloError        = errorPrefix + "Non-integer argument(s) passed for modulo"
	typeMismatchError        = errorPrefix + "Types mismatch"
)

func formatError(format string, function string, values ...interface{}) error {
	args := []interface{}{function}
	args = append(args, values...)
	return fmt.Errorf(format, args...)
}
