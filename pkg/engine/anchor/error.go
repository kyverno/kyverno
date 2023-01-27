package anchor

import (
	"errors"
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

// ValidateAnchorError represents the error type of validation anchors
type ValidateAnchorError struct {
	err     anchorError
	message string
}

// newNegationAnchorError returns a new instance of NegationAnchorError
func newNegationAnchorError(msg string) ValidateAnchorError {
	return ValidateAnchorError{
		err:     negationAnchorErr,
		message: fmt.Sprintf("%s: %s", negationAnchorErrMsg, msg),
	}
}

// newConditionalAnchorError returns a new instance of ConditionalAnchorError
func newConditionalAnchorError(msg string) ValidateAnchorError {
	return ValidateAnchorError{
		err:     conditionalAnchorErr,
		message: fmt.Sprintf("%s: %s", conditionalAnchorErrMsg, msg),
	}
}

// newGlobalAnchorError returns a new instance of GlobalAnchorError
func newGlobalAnchorError(msg string) ValidateAnchorError {
	return ValidateAnchorError{
		err:     globalAnchorErr,
		message: fmt.Sprintf("%s: %s", globalAnchorErrMsg, msg),
	}
}

// IsNegationAnchorError checks if error message has negation anchor error string
func IsNegationAnchorError(msg string) bool {
	return strings.Contains(msg, negationAnchorErrMsg)
}

// IsConditionalAnchorError checks if error message has conditional anchor error string
func IsConditionalAnchorError(msg string) bool {
	return strings.Contains(msg, conditionalAnchorErrMsg)
}

// IsGlobalAnchorError checks if error message has global anchor error string
func IsGlobalAnchorError(msg string) bool {
	return strings.Contains(msg, globalAnchorErrMsg)
}

// // IsNegationAnchorError checks if the error is a negation anchor error
// func (e ValidateAnchorError) IsNegationAnchorError() bool {
// 	return e.Err == NegationAnchorErr
// }

// // IsConditionAnchorError checks if the error is a conditional anchor error
// func (e ValidateAnchorError) IsConditionAnchorError() bool {
// 	return e.Err == ConditionalAnchorErr
// }

// // IsGlobalAnchorError checks if the error is a global anchor error
// func (e ValidateAnchorError) IsGlobalAnchorError() bool {
// 	return e.Err == GlobalAnchorErr
// }

// // IsNil checks if the error isn't populated
// func (e ValidateAnchorError) IsNil() bool {
// 	return e == ValidateAnchorError{}
// }

// Error returns an error instance of the anchor error
func (e ValidateAnchorError) Error() error {
	return errors.New(e.message)
}
