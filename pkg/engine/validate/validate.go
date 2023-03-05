package validate

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/anchor"
	"github.com/kyverno/kyverno/pkg/engine/pattern"
	"github.com/kyverno/kyverno/pkg/engine/wildcards"
	"go.uber.org/multierr"
)

type PatternError struct {
	Err  error
	Path string
	Skip bool
}

func (e *PatternError) Error() string {
	if e.Err == nil {
		return ""
	}

	return e.Err.Error()
}

// MatchPattern is a start of element-by-element pattern validation process.
// It assumes that validation is started from root, so "/" is passed
func MatchPattern(logger logr.Logger, resource, pattern interface{}) error {
	// newAnchorMap - to check anchor key has values
	ac := anchor.NewAnchorMap()
	elemPath, err := validateResourceElement(logger, resource, pattern, pattern, "/", ac)
	if err != nil {
		if skip(err) {
			logger.V(2).Info("resource skipped", "reason", ac.AnchorError.Error())
			return &PatternError{err, "", true}
		}

		if fail(err) {
			logger.V(2).Info("failed to apply rule on resource", "msg", ac.AnchorError.Error())
			return &PatternError{err, elemPath, false}
		}

		// check if an anchor defined in the policy rule is missing in the resource
		if ac.KeysAreMissing() {
			logger.V(3).Info("missing anchor in resource")
			return &PatternError{err, "", false}
		}

		return &PatternError{err, elemPath, false}
	}

	return nil
}

func skip(err error) bool {
	// if conditional or global anchors report errors, the rule does not apply to the resource
	return anchor.IsConditionalAnchorError(err) || anchor.IsGlobalAnchorError(err)
}

func fail(err error) bool {
	// if negation anchors report errors, the rule will fail
	return anchor.IsNegationAnchorError(err)
}

// validateResourceElement detects the element type (map, array, nil, string, int, bool, float)
// and calls corresponding handler
// Pattern tree and resource tree can have different structure. In this case validation fails
func validateResourceElement(log logr.Logger, resourceElement, patternElement, originPattern interface{}, path string, ac *anchor.AnchorMap) (string, error) {
	switch typedPatternElement := patternElement.(type) {
	// map
	case map[string]interface{}:
		typedResourceElement, ok := resourceElement.(map[string]interface{})
		if !ok {
			log.V(4).Info("Pattern and resource have different structures.", "path", path, "expected", fmt.Sprintf("%T", patternElement), "current", fmt.Sprintf("%T", resourceElement))
			return path, fmt.Errorf("pattern and resource have different structures. Path: %s. Expected %T, found %T", path, patternElement, resourceElement)
		}
		// CheckAnchorInResource - check anchor key exists in resource and update the AnchorKey fields.
		ac.CheckAnchorInResource(typedPatternElement, typedResourceElement)
		return validateMap(log, typedResourceElement, typedPatternElement, originPattern, path, ac)
	// array
	case []interface{}:
		typedResourceElement, ok := resourceElement.([]interface{})
		if !ok {
			log.V(4).Info("Pattern and resource have different structures.", "path", path, "expected", fmt.Sprintf("%T", patternElement), "current", fmt.Sprintf("%T", resourceElement))
			return path, fmt.Errorf("validation rule failed at path %s, resource does not satisfy the expected overlay pattern", path)
		}
		return validateArray(log, typedResourceElement, typedPatternElement, originPattern, path, ac)
	// elementary values
	case string, float64, int, int64, bool, nil:
		/*Analyze pattern */

		switch resource := resourceElement.(type) {
		case []interface{}:
			for _, res := range resource {
				if !pattern.Validate(log, res, patternElement) {
					return path, fmt.Errorf("resource value '%v' does not match '%v' at path %s", resourceElement, patternElement, path)
				}
			}
			return "", nil
		default:
			if !pattern.Validate(log, resourceElement, patternElement) {
				return path, fmt.Errorf("resource value '%v' does not match '%v' at path %s", resourceElement, patternElement, path)
			}
		}

	default:
		log.V(4).Info("Pattern contains unknown type", "path", path, "current", fmt.Sprintf("%T", patternElement))
		return path, fmt.Errorf("failed at '%s', pattern contains unknown type", path)
	}
	return "", nil
}

// If validateResourceElement detects map element inside resource and pattern trees, it goes to validateMap
// For each element of the map we must detect the type again, so we pass these elements to validateResourceElement
func validateMap(log logr.Logger, resourceMap, patternMap map[string]interface{}, origPattern interface{}, path string, ac *anchor.AnchorMap) (string, error) {
	patternMap = wildcards.ExpandInMetadata(patternMap, resourceMap)
	// check if there is anchor in pattern
	// Phase 1 : Evaluate all the anchors
	// Phase 2 : Evaluate non-anchors
	anchors, resources := anchor.GetAnchorsResourcesFromMap(patternMap)

	keys := make([]string, 0, len(anchors))
	for k := range anchors {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Evaluate anchors
	for _, key := range keys {
		patternElement := anchors[key]
		// get handler for each pattern in the pattern
		// - Conditional
		// - Existence
		// - Equality
		handler := anchor.CreateElementHandler(key, patternElement, path)
		handlerPath, err := handler.Handle(validateResourceElement, resourceMap, origPattern, ac)
		// if there are resource values at same level, then anchor acts as conditional instead of a strict check
		// but if there are none then it's an if-then check
		if err != nil {
			// If global anchor fails then we don't process the resource
			return handlerPath, err
		}
	}

	// Evaluate resources
	// getSortedNestedAnchorResource - keeps the anchor key to start of the list
	sortedResourceKeys := getSortedNestedAnchorResource(resources)
	for e := sortedResourceKeys.Front(); e != nil; e = e.Next() {
		key := e.Value.(string)
		handler := anchor.CreateElementHandler(key, resources[key], path)
		handlerPath, err := handler.Handle(validateResourceElement, resourceMap, origPattern, ac)
		if err != nil {
			return handlerPath, err
		}
	}

	return "", nil
}

func validateArray(log logr.Logger, resourceArray, patternArray []interface{}, originPattern interface{}, path string, ac *anchor.AnchorMap) (string, error) {
	if len(patternArray) == 0 {
		return path, fmt.Errorf("pattern Array empty")
	}

	switch typedPatternElement := patternArray[0].(type) {
	case map[string]interface{}:
		// This is special case, because maps in arrays can have anchors that must be
		// processed with the special way affecting the entire array
		elemPath, err := validateArrayOfMaps(log, resourceArray, typedPatternElement, originPattern, path, ac)
		if err != nil {
			return elemPath, err
		}
	case string, float64, int, int64, bool, nil:
		elemPath, err := validateResourceElement(log, resourceArray, typedPatternElement, originPattern, path, ac)
		if err != nil {
			return elemPath, err
		}
	default:
		// In all other cases - detect type and handle each array element with validateResourceElement
		if len(resourceArray) < len(patternArray) {
			return "", fmt.Errorf("validate Array failed, array length mismatch, resource Array len is %d and pattern Array len is %d", len(resourceArray), len(patternArray))
		}

		var applyCount int
		var skipErrors []error
		for i, patternElement := range patternArray {
			currentPath := path + strconv.Itoa(i) + "/"
			elemPath, err := validateResourceElement(log, resourceArray[i], patternElement, originPattern, currentPath, ac)
			if err != nil {
				if skip(err) {
					skipErrors = append(skipErrors, err)
					continue
				}

				return elemPath, err
			}

			applyCount++
		}

		if applyCount == 0 && len(skipErrors) > 0 {
			return path, &PatternError{
				Err:  multierr.Combine(skipErrors...),
				Path: path,
				Skip: true,
			}
		}
	}

	return "", nil
}

// validateArrayOfMaps gets anchors from pattern array map element, applies anchors logic
// and then validates each map due to the pattern
func validateArrayOfMaps(log logr.Logger, resourceMapArray []interface{}, patternMap map[string]interface{}, originPattern interface{}, path string, ac *anchor.AnchorMap) (string, error) {
	applyCount := 0
	skipErrors := make([]error, 0)
	for i, resourceElement := range resourceMapArray {
		// check the types of resource element
		// expect it to be a map, but can be anything ?:(
		currentPath := path + strconv.Itoa(i) + "/"
		returnPath, err := validateResourceElement(log, resourceElement, patternMap, originPattern, currentPath, ac)
		if err != nil {
			if skip(err) {
				skipErrors = append(skipErrors, err)
				continue
			}

			return returnPath, err
		}

		applyCount++
	}

	if applyCount == 0 && len(skipErrors) > 0 {
		return path, &PatternError{
			Err:  multierr.Combine(skipErrors...),
			Path: path,
			Skip: true,
		}
	}

	return "", nil
}
