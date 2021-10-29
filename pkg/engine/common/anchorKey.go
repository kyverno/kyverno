package common

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kyverno/kyverno/pkg/engine/anchor/common"
)

// IsConditionalAnchorError checks if error message has conditional anchor error string
func IsConditionalAnchorError(msg string) bool {
	return strings.Contains(msg, ConditionalAnchorErrMsg)
}

// IsGlobalAnchorError checks if error message has global anchor error string
func IsGlobalAnchorError(msg string) bool {
	return strings.Contains(msg, GlobalAnchorErrMsg)
}

// NewConditionalAnchorError returns a new instance of ConditionalAnchorError
func NewConditionalAnchorError(msg string) ValidateAnchorError {
	return ValidateAnchorError{
		Err:     ConditionalAnchorErr,
		Message: fmt.Sprintf("%s: %s", ConditionalAnchorErrMsg, msg),
	}
}

// IsConditionAnchorError checks if the error is a conditional anchor error
func (e ValidateAnchorError) IsConditionAnchorError() bool {
	return e.Err == ConditionalAnchorErr
}

// NewGlobalAnchorError returns a new instance of GlobalAnchorError
func NewGlobalAnchorError(msg string) ValidateAnchorError {
	return ValidateAnchorError{
		Err:     GlobalAnchorErr,
		Message: fmt.Sprintf("%s: %s", GlobalAnchorErrMsg, msg),
	}
}

// IsGlobalAnchorError checks if the error is a global anchor error
func (e ValidateAnchorError) IsGlobalAnchorError() bool {
	return e.Err == GlobalAnchorErr
}

// IsNil checks if the error isn't populated
func (e ValidateAnchorError) IsNil() bool {
	return e == ValidateAnchorError{}
}

// Error returns an error instance of the anchor error
func (e ValidateAnchorError) Error() error {
	return errors.New(e.Message)
}

// AnchorError is the const specification of anchor errors
type AnchorError int

const (
	// ConditionalAnchorErr refers to condition violation
	ConditionalAnchorErr AnchorError = iota

	// GlobalAnchorErr refers to global condition violation
	GlobalAnchorErr
)

// ValidateAnchorError represents the error type of validation anchors
type ValidateAnchorError struct {
	Err     AnchorError
	Message string
}

// ConditionalAnchorErrMsg - the error message for conditional anchor error
var ConditionalAnchorErrMsg = "conditional anchor mismatch"

// GlobalAnchorErrMsg - the error message for global anchor error
var GlobalAnchorErrMsg = "global anchor mismatch"

// AnchorKey - contains map of anchors
type AnchorKey struct {
	// anchorMap - for each anchor key in the patterns it will maintains information if the key exists in the resource
	// if anchor key of the pattern exists in the resource then (key)=true else (key)=false
	anchorMap map[string]bool
	// AnchorError - used in validate to break execution of the recursion when if condition fails
	AnchorError ValidateAnchorError
}

// NewAnchorMap -initialize anchorMap
func NewAnchorMap() *AnchorKey {
	return &AnchorKey{anchorMap: make(map[string]bool)}
}

// IsAnchorError - if any of the anchor key doesn't exists in the resource then it will return true
// if any of (key)=false then return IsAnchorError() as true
// if all the keys exists in the pattern exists in resource then return IsAnchorError() as false
func (ac *AnchorKey) IsAnchorError() bool {
	for _, v := range ac.anchorMap {
		if !v {
			return true
		}
	}
	return false
}

// CheckAnchorInResource checks if condition anchor key has values
func (ac *AnchorKey) CheckAnchorInResource(pattern interface{}, resource interface{}) {
	switch typed := pattern.(type) {
	case map[string]interface{}:
		for key := range typed {
			if common.IsConditionAnchor(key) || common.IsExistenceAnchor(key) || common.IsNegationAnchor(key) {
				val, ok := ac.anchorMap[key]
				if !ok {
					ac.anchorMap[key] = false
				} else if ok && val {
					continue
				}
				if doesAnchorsKeyHasValue(key, resource) {
					ac.anchorMap[key] = true
				}
			}
		}
	}
}

// Checks if anchor key has value in resource
func doesAnchorsKeyHasValue(key string, resource interface{}) bool {
	akey, _ := common.RemoveAnchor(key)
	switch typed := resource.(type) {
	case map[string]interface{}:
		if _, ok := typed[akey]; ok {
			return true
		}
		return false
	case []interface{}:
		for _, value := range typed {
			if doesAnchorsKeyHasValue(key, value) {
				return true
			}
		}
		return false
	default:
		return false
	}
}
