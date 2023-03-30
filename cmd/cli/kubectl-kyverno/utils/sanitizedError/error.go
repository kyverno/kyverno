package sanitizederror

import (
	"fmt"
	"strings"
)

type customError struct {
	message string
}

func (c customError) Error() string {
	return c.message
}

func New(msg string) error {
	return customError{message: msg}
}

func NewWithErrors(message string, errors []error) error {
	bldr := strings.Builder{}
	bldr.WriteString(message + "\n")
	for _, err := range errors {
		bldr.WriteString(err.Error() + "\n")
	}

	return customError{message: bldr.String()}
}

// NewWithError creates a new sanitized error with given message and error
func NewWithError(message string, err error) error {
	if err == nil {
		return customError{message: message}
	}

	msg := fmt.Sprintf("%s \nCause: %s", message, err.Error())
	return customError{message: msg}
}

// IsErrorSanitized checks if the error is sanitized error
func IsErrorSanitized(err error) bool {
	if _, ok := err.(customError); !ok {
		return false
	}
	return true
}
