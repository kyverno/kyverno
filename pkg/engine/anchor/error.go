package anchor

import (
	"fmt"
	"strings"
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

// isError checks if error matches the given error type
func isError(err error, code anchorError, msg string) bool {
	if err != nil {
		if t, ok := err.(validateAnchorError); ok {
			return t.err == code
		} else {
			// TODO: we shouldn't need this, error is not properly propagated
			return strings.Contains(err.Error(), msg)
		}
	}
	return false
}

// IsNegationAnchorError checks if error is a negation anchor error
func IsNegationAnchorError(err error) bool {
	return isError(err, negationAnchorErr, negationAnchorErrMsg)
}

// IsConditionalAnchorError checks if error is a conditional anchor error
func IsConditionalAnchorError(err error) bool {
	return isError(err, conditionalAnchorErr, conditionalAnchorErrMsg)
}

// IsGlobalAnchorError checks if error is a global global anchor error
func IsGlobalAnchorError(err error) bool {
	return isError(err, globalAnchorErr, globalAnchorErrMsg)
}
