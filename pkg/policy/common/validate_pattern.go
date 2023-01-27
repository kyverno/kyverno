package common

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/kyverno/kyverno/pkg/engine/anchor"
)

// ValidatePattern validates the pattern
func ValidatePattern(patternElement interface{}, path string, isSupported func(anchor.AnchorHandler) bool) (string, error) {
	switch typedPatternElement := patternElement.(type) {
	case map[string]interface{}:
		return validateMap(typedPatternElement, path, isSupported)
	case []interface{}:
		return validateArray(typedPatternElement, path, isSupported)
	case string, float64, int, int64, bool, nil:
		// TODO: check operator
		return "", nil
	default:
		return path, fmt.Errorf("error at '%s', pattern contains unknown type", path)
	}
}

func validateMap(patternMap map[string]interface{}, path string, isSupported func(anchor.AnchorHandler) bool) (string, error) {
	// check if anchors are defined
	for key, value := range patternMap {
		// if key is anchor
		// check regex () -> this is anchor
		// ()
		// single char ()
		re, err := regexp.Compile(`^.?\(.+\)$`)
		if err != nil {
			return path + "/" + key, fmt.Errorf("unable to parse the field %s: %v", key, err)
		}

		matched := re.MatchString(key)
		// check the type of anchor
		if matched {
			// some type of anchor
			// check if valid anchor
			if !checkAnchors(key, isSupported) {
				return path + "/" + key, fmt.Errorf("unsupported anchor %s", key)
			}

			// addition check for existence anchor
			// value must be of type list
			if anchor.IsExistenceAnchor(key) {
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

func validateArray(patternArray []interface{}, path string, isSupported func(anchor.AnchorHandler) bool) (string, error) {
	for i, patternElement := range patternArray {
		currentPath := path + strconv.Itoa(i) + "/"
		// lets validate the values now :)
		if errPath, err := ValidatePattern(patternElement, currentPath, isSupported); err != nil {
			return errPath, err
		}
	}
	return "", nil
}

func checkAnchors(key string, isSupported func(anchor.AnchorHandler) bool) bool {
	if isSupported == nil {
		return false
	}
	a := anchor.ParseAnchor(key)
	return a != nil && isSupported(*a)
}
