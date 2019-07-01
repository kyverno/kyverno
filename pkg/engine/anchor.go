package engine

import (
	"fmt"
	"strconv"
)

// CreateAnchorHandler is a factory that create anchor handlers
func CreateAnchorHandler(anchor string, pattern interface{}, path string) ValidationAnchorHandler {
	switch {
	case isConditionAnchor(anchor):
		return NewConditionAnchorValidationHandler(anchor, pattern, path)
	case isExistanceAnchor(anchor):
		return NewExistanceAnchorValidationHandler(anchor, pattern, path)
	default:
		return NewNoAnchorValidationHandler(path)
	}
}

// ValidationAnchorHandler is an interface that represents
// a family of anchor handlers for array of maps
// resourcePart must be an array of dictionaries
// patternPart must be a dictionary with anchors
type ValidationAnchorHandler interface {
	Handle(resourcePart []interface{}, patternPart map[string]interface{}, originPattern interface{}) error
}

// NoAnchorValidationHandler just calls validateMap
// because no anchors were found in the pattern map
type NoAnchorValidationHandler struct {
	path string
}

// NewNoAnchorValidationHandler creates new instance of
// NoAnchorValidationHandler
func NewNoAnchorValidationHandler(path string) ValidationAnchorHandler {
	return &NoAnchorValidationHandler{
		path: path,
	}
}

// Handle performs validation in context of NoAnchorValidationHandler
func (navh *NoAnchorValidationHandler) Handle(resourcePart []interface{}, patternPart map[string]interface{}, originPattern interface{}) error {

	for i, resourceElement := range resourcePart {
		currentPath := navh.path + strconv.Itoa(i) + "/"

		typedResourceElement, ok := resourceElement.(map[string]interface{})
		if !ok {
			return fmt.Errorf("Pattern and resource have different structures. Path: %s. Expected %T, found %T", currentPath, patternPart, resourceElement)
		}

		err := validateMap(typedResourceElement, patternPart, originPattern, currentPath)
		if err != nil {
			return err
		}
	}

	return nil
}

// ConditionAnchorValidationHandler performs
// validation only for array elements that
// pass condition in the anchor
// (key): value
type ConditionAnchorValidationHandler struct {
	anchor  string
	pattern interface{}
	path    string
}

// NewConditionAnchorValidationHandler creates new instance of
// NoAnchorValidationHandler
func NewConditionAnchorValidationHandler(anchor string, pattern interface{}, path string) ValidationAnchorHandler {
	return &ConditionAnchorValidationHandler{
		anchor:  anchor,
		pattern: pattern,
		path:    path,
	}
}

// Handle performs validation in context of ConditionAnchorValidationHandler
func (cavh *ConditionAnchorValidationHandler) Handle(resourcePart []interface{}, patternPart map[string]interface{}, originPattern interface{}) error {
	_, handlingResult := handleConditionCases(resourcePart, patternPart, cavh.anchor, cavh.pattern, cavh.path, originPattern)

	return handlingResult
}

// ExistanceAnchorValidationHandler performs
// validation only for array elements that
// pass condition in the anchor
// AND requires an existance of at least one
// element that passes this condition
// ^(key): value
type ExistanceAnchorValidationHandler struct {
	anchor  string
	pattern interface{}
	path    string
}

// NewExistanceAnchorValidationHandler creates new instance of
// NoAnchorValidationHandler
func NewExistanceAnchorValidationHandler(anchor string, pattern interface{}, path string) ValidationAnchorHandler {
	return &ExistanceAnchorValidationHandler{
		anchor:  anchor,
		pattern: pattern,
		path:    path,
	}
}

// Handle performs validation in context of ExistanceAnchorValidationHandler
func (eavh *ExistanceAnchorValidationHandler) Handle(resourcePart []interface{}, patternPart map[string]interface{}, originPattern interface{}) error {
	anchoredEntries, err := handleConditionCases(resourcePart, patternPart, eavh.anchor, eavh.pattern, eavh.path, originPattern)
	if err != nil {
		return err
	}
	if 0 == anchoredEntries {
		return fmt.Errorf("Existance anchor %s used, but no suitable entries were found", eavh.anchor)
	}

	return nil
}

// check if array element fits the anchor
func checkForAnchorCondition(anchor string, pattern interface{}, resourceMap map[string]interface{}) bool {
	anchorKey := removeAnchor(anchor)

	if value, ok := resourceMap[anchorKey]; ok {
		return ValidateValueWithPattern(value, pattern)
	}

	return false
}

// both () and ^() are checking conditions and have a lot of similar logic
// the only difference is that ^() requires existace of one element
// anchoredEntries var counts this occurences.
func handleConditionCases(resourcePart []interface{}, patternPart map[string]interface{}, anchor string, pattern interface{}, path string, originPattern interface{}) (int, error) {
	anchoredEntries := 0

	for i, resourceElement := range resourcePart {
		currentPath := path + strconv.Itoa(i) + "/"

		typedResourceElement, ok := resourceElement.(map[string]interface{})
		if !ok {
			return 0, fmt.Errorf("Pattern and resource have different structures. Path: %s. Expected %T, found %T", currentPath, patternPart, resourceElement)
		}

		if !checkForAnchorCondition(anchor, pattern, typedResourceElement) {
			continue
		}

		anchoredEntries++
		err := validateMap(typedResourceElement, patternPart, originPattern, currentPath)
		if err != nil {
			return 0, err
		}
	}

	return anchoredEntries, nil
}
