package anchor

import (
	"errors"
	"fmt"
)

// anchorError is the const specification of anchor errors
type anchorError int

const (
	// conditionalAnchorErr refers to condition violation
	conditionalAnchorErr anchorError = iota
	// globalAnchorErr refers to global condition violation
	globalAnchorErr
	// negationAnchorErr refers to negation violation
	negationAnchorErr
)

const (
	// negationAnchorErrMsg - the error message for negation anchor error
	negationAnchorErrMsg = "negation anchor matched in resource"
	// conditionalAnchorErrMsg - the error message for conditional anchor error
	conditionalAnchorErrMsg = "conditional anchor mismatch"
	// globalAnchorErrMsg - the error message for global anchor error
	globalAnchorErrMsg = "global anchor mismatch"
)

// validateAnchorError represents the error type of validation anchors
type validateAnchorError struct {
	err     anchorError
	message string
	// cause is the underlying error this anchor error was built from, if any.
	// It is exposed through Unwrap so that errors.As can walk the chain.
	cause error
}

// Error implements error interface
func (e validateAnchorError) Error() string {
	return e.message
}

// Unwrap implements the errors.Unwrap contract, returning the underlying cause
// this anchor error was built from. It returns nil for anchor errors that are
// not derived from another error.
func (e validateAnchorError) Unwrap() error {
	return e.cause
}

// newNegationAnchorError returns a new instance of validateAnchorError
func newValidateAnchorError(err anchorError, prefix, msg string) validateAnchorError {
	return validateAnchorError{
		err:     err,
		message: fmt.Sprintf("%s: %s", prefix, msg),
	}
}

// wrapValidateAnchorError returns a new instance of validateAnchorError that
// keeps a reference to the error it was built from. The rendered message is
// identical to newValidateAnchorError(err, prefix, cause.Error()).
func wrapValidateAnchorError(err anchorError, prefix string, cause error) validateAnchorError {
	return validateAnchorError{
		err:     err,
		message: fmt.Sprintf("%s: %s", prefix, cause.Error()),
		cause:   cause,
	}
}

// newNegationAnchorError returns a new instance of NegationAnchorError
func newNegationAnchorError(msg string) validateAnchorError {
	return newValidateAnchorError(negationAnchorErr, negationAnchorErrMsg, msg)
}

// newConditionalAnchorError returns a new instance of ConditionalAnchorError
func newConditionalAnchorError(msg string) validateAnchorError {
	return newValidateAnchorError(conditionalAnchorErr, conditionalAnchorErrMsg, msg)
}

// newGlobalAnchorError returns a new instance of GlobalAnchorError
func newGlobalAnchorError(msg string) validateAnchorError {
	return newValidateAnchorError(globalAnchorErr, globalAnchorErrMsg, msg)
}

// wrapConditionalAnchorError returns a conditional anchor error wrapping cause
func wrapConditionalAnchorError(cause error) validateAnchorError {
	return wrapValidateAnchorError(conditionalAnchorErr, conditionalAnchorErrMsg, cause)
}

// wrapGlobalAnchorError returns a global anchor error wrapping cause
func wrapGlobalAnchorError(cause error) validateAnchorError {
	return wrapValidateAnchorError(globalAnchorErr, globalAnchorErrMsg, cause)
}

// isError checks if err, or any error it wraps, is an anchor error of the given
// type. Classification is based on the error type only: the error message is
// never inspected, so an unrelated error cannot be misclassified just because
// its text happens to contain an anchor error message.
func isError(err error, code anchorError) bool {
	if err == nil {
		return false
	}
	var target validateAnchorError
	if errors.As(err, &target) {
		return target.err == code
	}
	return false
}

// IsNegationAnchorError checks if error is a negation anchor error
func IsNegationAnchorError(err error) bool {
	return isError(err, negationAnchorErr)
}

// IsConditionalAnchorError checks if error is a conditional anchor error
func IsConditionalAnchorError(err error) bool {
	return isError(err, conditionalAnchorErr)
}

// IsGlobalAnchorError checks if error is a global anchor error
func IsGlobalAnchorError(err error) bool {
	return isError(err, globalAnchorErr)
}
