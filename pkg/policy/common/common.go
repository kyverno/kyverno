package common

import (
	"fmt"
	"regexp"
	"strconv"

	commonAnchors "github.com/nirmata/kyverno/pkg/engine/anchor/common"
)

//ValidatePattern validates the pattern
func ValidatePattern(patternElement interface{}, path string, supportedAnchors []commonAnchors.IsAnchor) (string, error) {
	switch typedPatternElement := patternElement.(type) {
	case map[string]interface{}:
		return validateMap(typedPatternElement, path, supportedAnchors)
	case []interface{}:
		return validateArray(typedPatternElement, path, supportedAnchors)
	case string, float64, int, int64, bool, nil:
		//TODO? check operator
		return "", nil
	default:
		return path, fmt.Errorf("Validation rule failed at '%s', pattern contains unknown type", path)
	}
}
func validateMap(patternMap map[string]interface{}, path string, supportedAnchors []commonAnchors.IsAnchor) (string, error) {
	// check if anchors are defined
	for key, value := range patternMap {
		// if key is anchor
		// check regex () -> this is anchor
		// ()
		// single char ()
		re, err := regexp.Compile(`^.?\(.+\)$`)
		if err != nil {
			return path + "/" + key, fmt.Errorf("Unable to parse the field %s: %v", key, err)
		}

		matched := re.MatchString(key)
		// check the type of anchor
		if matched {
			// some type of anchor
			// check if valid anchor
			if !checkAnchors(key, supportedAnchors) {
				return path + "/" + key, fmt.Errorf("Unsupported anchor %s", key)
			}

			// addition check for existence anchor
			// value must be of type list
			if commonAnchors.IsExistenceAnchor(key) {
				typedValue, ok := value.([]interface{})
				if !ok {
					return path + "/" + key, fmt.Errorf("Existence anchor should have value of type list")
				}
				// validate there is only one entry in the list
				if len(typedValue) == 0 || len(typedValue) > 1 {
					return path + "/" + key, fmt.Errorf("Existence anchor: single value expected, multiple specified")
				}
			}
		}
		// lets validate the values now :)
		if errPath, err := ValidatePattern(value, path+"/"+key, supportedAnchors); err != nil {
			return errPath, err
		}
	}
	return "", nil
}

func validateArray(patternArray []interface{}, path string, supportedAnchors []commonAnchors.IsAnchor) (string, error) {
	for i, patternElement := range patternArray {
		currentPath := path + strconv.Itoa(i) + "/"
		// lets validate the values now :)
		if errPath, err := ValidatePattern(patternElement, currentPath, supportedAnchors); err != nil {
			return errPath, err
		}
	}
	return "", nil
}

func checkAnchors(key string, supportedAnchors []commonAnchors.IsAnchor) bool {
	for _, f := range supportedAnchors {
		if f(key) {
			return true
		}
	}
	return false
}
