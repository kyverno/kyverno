package common

import (
	"fmt"
	"strconv"

	"github.com/kyverno/kyverno/pkg/engine/anchor"
)

// ValidatePattern validates the pattern
func ValidatePattern(patternElement interface{}, path string, isSupported func(anchor.Anchor) bool) (string, error) {
	switch typedPatternElement := patternElement.(type) {
	case map[string]interface{}:
		return validateMap(typedPatternElement, path, isSupported)
	case []interface{}:
		return validateArray(typedPatternElement, path, isSupported)
	case string:
		return validateStringPattern(typedPatternElement, path)
	case float64, int, int64:
		return validateNumericPattern(typedPatternElement, path)
	case bool:
		return validateBoolPattern(path)
	case nil:
		return validateNilPattern(path)
	default:
		return path, fmt.Errorf("error at '%s', pattern contains unknown type", path)
	}
}

func validateMap(patternMap map[string]interface{}, path string, isSupported func(anchor.Anchor) bool) (string, error) {
	// check if anchors are defined
	for key, value := range patternMap {
		// if key is anchor
		a := anchor.Parse(key)
		// check the type of anchor
		if a != nil {
			// some type of anchor
			// check if valid anchor
			if !checkAnchors(a, isSupported) {
				return path + "/" + key, fmt.Errorf("unsupported anchor %s", key)
			}
			// addition check for existence anchor
			// value must be of type list
			if anchor.IsExistence(a) {
				typedValue, ok := value.([]interface{})
				if !ok {
					return path + "/" + key, fmt.Errorf("existence anchor should have value of type list")
				}
				// validate that there is at least one entry in the list
				if len(typedValue) == 0 {
					return path + "/" + key, fmt.Errorf("existence anchor: should have at least one value")
				}
			}
		}
		// lets validate the values now :)
		if errPath, err := ValidatePattern(value, path+"/"+key, isSupported); err != nil {
			return errPath, err
		}
	}
	return "", nil
}

func validateArray(patternArray []interface{}, path string, isSupported func(anchor.Anchor) bool) (string, error) {
	for i, patternElement := range patternArray {
		currentPath := path + strconv.Itoa(i) + "/"
		// lets validate the values now :)
		if errPath, err := ValidatePattern(patternElement, currentPath, isSupported); err != nil {
			return errPath, err
		}
	}
	return "", nil
}

func checkAnchors(a anchor.Anchor, isSupported func(anchor.Anchor) bool) bool {
	if isSupported == nil {
		return false
	}
	return isSupported(a)
}

func validateStringPattern(pattern string, path string) (string, error) {
	// All operators are valid for strings, no additional validation needed
	// The operator parsing and validation happens at runtime in pkg/engine/pattern
	return "", nil
}

func validateNumericPattern(pattern interface{}, path string) (string, error) {
	// Numeric patterns (int, int64, float64) are literal values
	// They don't contain operators as operators are only in string patterns
	return "", nil
}

func validateBoolPattern(path string) (string, error) {
	// Boolean patterns are literal values (true/false)
	// They don't contain operators
	return "", nil
}

func validateNilPattern(path string) (string, error) {
	// Nil patterns don't contain operators
	return "", nil
}
