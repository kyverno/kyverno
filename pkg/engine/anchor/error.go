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

// newNegationAnchorError returns a new instance of NegationAnchorError
func newNegationAnchorError(msg string) validateAnchorError {
	return validateAnchorError{
		err:     negationAnchorErr,
		message: fmt.Sprintf("%s: %s", negationAnchorErrMsg, msg),
	}
}

// newConditionalAnchorError returns a new instance of ConditionalAnchorError
func newConditionalAnchorError(msg string) validateAnchorError {
	return validateAnchorError{
		err:     conditionalAnchorErr,
		message: fmt.Sprintf("%s: %s", conditionalAnchorErrMsg, msg),
	}
}

// newGlobalAnchorError returns a new instance of GlobalAnchorError
func newGlobalAnchorError(msg string) validateAnchorError {
	return validateAnchorError{
		err:     globalAnchorErr,
		message: fmt.Sprintf("%s: %s", globalAnchorErrMsg, msg),
	}
}

// IsNegationAnchorError checks if error is a negation anchor error
func IsNegationAnchorError(err error) bool {
	if err != nil {
		if t, ok := err.(validateAnchorError); ok {
			return t.err == negationAnchorErr
		}
	}
	return false
}

// IsConditionalAnchorError checks if error is a conditional anchor error
func IsConditionalAnchorError(err error) bool {
	if err != nil {
		if t, ok := err.(validateAnchorError); ok {
			return t.err == conditionalAnchorErr
		}
	}
	return false
}

// IsGlobalAnchorError checks if error is a global global anchor error
func IsGlobalAnchorError(err error) bool {
	if err != nil {
		if t, ok := err.(validateAnchorError); ok {
			return t.err == globalAnchorErr
		}
	}
	return false
}
