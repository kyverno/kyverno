package validate

import (
	"fmt"
	"strconv"
)

type ValidationFailureReason int

const (
	PathNotPresent ValidationFailureReason = iota
	Rulefailure
)

// convertToString converts value to string
func convertToString(value interface{}) (string, error) {
	switch typed := value.(type) {
	case string:
		return string(typed), nil
	case float64:
		return fmt.Sprintf("%f", typed), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case int:
		return strconv.Itoa(typed), nil
	default:
		return "", fmt.Errorf("Could not convert %T to string", value)
	}
}

func getRawKeyIfWrappedWithAttributes(str string) string {
	if len(str) < 2 {
		return str
	}

	if str[0] == '(' && str[len(str)-1] == ')' {
		return str[1 : len(str)-1]
	} else if (str[0] == '$' || str[0] == '^' || str[0] == '+' || str[0] == '=') && (str[1] == '(' && str[len(str)-1] == ')') {
		return str[2 : len(str)-1]
	} else {
		return str
	}
}

type ValidationError struct {
	StatusCode ValidationFailureReason
	ErrorMsg   string
}

// newValidatePatternError returns an validation error using the ValidationFailureReason and errorMsg
func newValidatePatternError(reason ValidationFailureReason, msg string) ValidationError {
	return ValidationError{StatusCode: reason, ErrorMsg: msg}
}
