package sanitizedError

import "fmt"

type customError struct {
	message string
}

func (c customError) Error() string {
	return c.message
}

func New(message string) error {
	return customError{message: message}
}

func NewWithError(message string, err error) error {
	if err == nil {
		return customError{message: message}
	}

	msg := fmt.Sprintf("%s \nCause: %s", message, err.Error())
	return customError{message: msg}
}

func IsErrorSanitized(err error) bool {
	if _, ok := err.(customError); !ok {
		return false
	}
	return true
}
