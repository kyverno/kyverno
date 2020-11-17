package sanitizederror

import "fmt"

type customError struct {
	message string
}

func (c customError) Error() string {
	return c.message
}

// New creates a new sanitized error with given message
func New(message string) error {
	return customError{message: message}
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
