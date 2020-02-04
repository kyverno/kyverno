package validate

import (
	"fmt"
	"strconv"
)

//ValidationFailureReason defeins type for Validation Failure reason
type ValidationFailureReason int

const (
	// PathNotPresent if path is not present
	PathNotPresent ValidationFailureReason = iota
	// Rulefailure if the rule failed
	Rulefailure
	// OtherError if there is any other type of error
	OtherError
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

//ValidationError stores error for validation error
type ValidationError struct {
	StatusCode ValidationFailureReason
	ErrorMsg   string
}

// newValidatePatternError returns an validation error using the ValidationFailureReason and errorMsg
func newValidatePatternError(reason ValidationFailureReason, msg string) ValidationError {
	return ValidationError{StatusCode: reason, ErrorMsg: msg}
}
