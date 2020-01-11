package engine

import "fmt"

type codeKey int

const (
	conditionFailure codeKey = iota
	conditionNotPresent
	overlayFailure
)

type overlayError struct {
	statusCode codeKey
	errorMsg   string
}

// newOverlayError returns an overlay error using the statusCode and errorMsg
func newOverlayError(code codeKey, msg string) overlayError {
	return overlayError{statusCode: code, errorMsg: msg}
}

// StatusCode returns the codeKey wrapped with status code of the overlay error
func (e overlayError) StatusCode() codeKey {
	return e.statusCode
}

// ErrorMsg returns the string representation of the overlay error message
func (e overlayError) ErrorMsg() string {
	return e.errorMsg
}

// Error returns the string representation of the overlay error
func (e overlayError) Error() string {
	return fmt.Sprintf("[overlayError:%v] %v", e.statusCode, e.errorMsg)
}
