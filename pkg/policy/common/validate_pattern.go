package common

import (
	"fmt"
	"strconv"
	"strings"

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
		// Validate operator syntax for string patterns
		return validateStringPattern(typedPatternElement, path)
	case float64, int, int64, bool, nil:
		return "", nil
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
				// validate that there is atleast one entry in the list
				if len(typedValue) == 0 {
					return path + "/" + key, fmt.Errorf("existence anchor: should have atleast one value")
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

// validateStringPattern validates operator syntax in string patterns
func validateStringPattern(pattern string, path string) (string, error) {
	// Only validate strings that actually look like operator patterns
	// Avoid false positives for regular strings that happen to contain dashes
	
	// Check for comparison operators at the beginning
	if strings.HasPrefix(pattern, ">=") || strings.HasPrefix(pattern, "<=") ||
		strings.HasPrefix(pattern, ">") || strings.HasPrefix(pattern, "<") {
		// These are valid comparison operators, no additional validation needed
		return "", nil
	}
	
	// Check for negation operators at the beginning
	if strings.HasPrefix(pattern, "!") {
		// Check for invalid !- syntax (should have format like "1!-10", not just "1!-")
		if strings.HasPrefix(pattern, "!-") {
			return path, fmt.Errorf("invalid operator syntax in pattern '%s': !- requires range format", pattern)
		}
		// Allow other negation patterns like "!disabled"
		return "", nil
	}
	
	// Check for range operators with more specific validation
	// Only flag as invalid if it looks like a malformed range operator
	if strings.Contains(pattern, "!-") && strings.HasSuffix(pattern, "!-") {
		return path, fmt.Errorf("invalid operator syntax in pattern '%s': !- requires range format", pattern)
	}
	
	// Check for trailing dash only if it appears to be an incomplete range operator
	// Pattern like "1-" or "value-" where it looks like an incomplete range
	if strings.HasSuffix(pattern, "-") && !strings.HasPrefix(pattern, "-") {
		// Check if this might be an incomplete numeric range
		beforeDash := strings.TrimSuffix(pattern, "-")
		// Only flag as error if it looks like it could be a numeric range
		if len(beforeDash) > 0 && (isNumericish(beforeDash) || strings.Contains(beforeDash, "Mi") || strings.Contains(beforeDash, "Gi") || strings.Contains(beforeDash, "Ki")) {
			return path, fmt.Errorf("invalid range operator syntax in pattern '%s'", pattern)
		}
	}
	
	return "", nil
}

// isNumericish checks if a string looks like it could be part of a numeric range
func isNumericish(s string) bool {
	// Check if string contains only digits, decimal point, or common units
	for _, r := range s {
		if !((r >= '0' && r <= '9') || r == '.' || r == 'm' || r == 'M' || r == 'k' || r == 'K' || r == 'g' || r == 'G') {
			return false
		}
	}
	return len(s) > 0
}
