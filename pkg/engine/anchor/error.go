package anchor

import (
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
}

// Error implements error interface
func (e validateAnchorError) Error() string {
	return e.message
}

// newNegationAnchorError returns a new instance of validateAnchorError
func newValidateAnchorError(err anchorError, prefix, msg string) validateAnchorError {
	return validateAnchorError{
		err:     err,
		message: fmt.Sprintf("%s: %s", prefix, msg),
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

// isError checks if any error in err's tree is an anchor error with the given code.
// It walks every branch (fmt.Errorf with %w, multierr, custom Unwrap methods) rather
// than stopping at the first anchor error, since combined errors can hold several.
func isError(err error, code anchorError) bool {
	switch e := err.(type) {
	case nil:
		return false
	case validateAnchorError:
		return e.err == code
	case interface{ Unwrap() []error }:
		for _, err := range e.Unwrap() {
			if isError(err, code) {
				return true
			}
		}
		return false
	case interface{ Unwrap() error }:
		return isError(e.Unwrap(), code)
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
