package engine

import (
	"fmt"
	"strconv"

	"github.com/golang/glog"
)

type ValidationHandler interface {
	Handle(resourceMap map[string]interface{}, originPattenr interface{}) (string, bool, error)
}

func CreateElementHandler(element string, pattern interface{}, path string) ValidationHandler {
	switch {
	case isConditionAnchor(element):
		return NewConditionAnchorHandler(element, pattern, path)
	case isExistanceAnchor(element):
		return NewExistanceHandler(element, pattern, path)
	default:
		return NewDefaultHandler(element, pattern, path)
	}
}

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

func NewDefaultHandler(element string, pattern interface{}, path string) ValidationHandler {
	return DefaultHandler{
		element: element,
		pattern: pattern,
		path:    path,
	}
}

type DefaultHandler struct {
	element string
	pattern interface{}
	path    string
}

func (dh DefaultHandler) Handle(resourceMap map[string]interface{}, originPattern interface{}) (string, bool, error) {
	// skip is used by existance anchor to not process further if condition is not satisfied
	skip := false
	currentPath := dh.path + dh.element + "/"
	if dh.pattern == "*" && resourceMap[dh.element] != nil {
		return "", skip, nil
	} else if dh.pattern == "*" && resourceMap[dh.element] == nil {
		return dh.path, skip, fmt.Errorf("Validation rule failed at %s, Field %s is not present", dh.path, dh.element)
	} else {
		path, err := validateResourceElement(resourceMap[dh.element], dh.pattern, originPattern, currentPath)
		if err != nil {
			return path, skip, err
		}
	}
	return "", skip, nil
}

func NewConditionAnchorHandler(anchor string, pattern interface{}, path string) ValidationHandler {
	return ConditionAnchorHandler{
		anchor:  anchor,
		pattern: pattern,
		path:    path,
	}
}

type ConditionAnchorHandler struct {
	anchor  string
	pattern interface{}
	path    string
}

func (ch ConditionAnchorHandler) Handle(resourceMap map[string]interface{}, originPattern interface{}) (string, bool, error) {
	// skip is used by existance anchor to not process further if condition is not satisfied
	skip := false
	var value interface{}
	var currentPath string
	var ok bool
	// check for anchor condition
	anchorSatisfied := func() bool {
		anchorKey := removeAnchor(ch.anchor)
		currentPath = ch.path + anchorKey + "/"
		// check if anchor is present in resource
		if value, ok = resourceMap[anchorKey]; ok {
			// if the key exists then we process its values
			return true
			// return ValidateValueWithPattern(value, ch.pattern)
		}
		return false
	}()

	if !anchorSatisfied {
		return "", skip, nil
	}

	path, err := validateResourceElement(value, ch.pattern, originPattern, currentPath)
	if err != nil {
		return path, skip, err
	}
	// evauluate the anchor and resource values
	// for key, element := range resourceMap {
	// 	currentPath := ch.path + key + "/"
	// 	if !ValidateValueWithPattern(element, ch.pattern) {
	// 		// the anchor does not match so ignore
	// 		continue
	// 	}
	// 	path, err := validateResourceElement(element, ch.pattern, originPattern, currentPath)
	// 	if err != nil {
	// 		return path, err
	// 	}
	// }
	return "", skip, nil
}

func NewExistanceHandler(anchor string, pattern interface{}, path string) ValidationHandler {
	return ExistanceHandler{
		anchor:  anchor,
		pattern: pattern,
		path:    path,
	}
}

type ExistanceHandler struct {
	anchor  string
	pattern interface{}
	path    string
}

func (eh ExistanceHandler) Handle(resourceMap map[string]interface{}, originPattern interface{}) (string, bool, error) {
	// skip is used by existance anchor to not process further if condition is not satisfied
	var value interface{}
	var currentPath string
	var ok bool
	// anchoredEntries := 0

	// check for anchor condition
	anchorSatisfied := func() bool {
		anchorKey := removeAnchor(eh.anchor)
		currentPath = eh.path + anchorKey + "/"
		// check if anchor is present in resource
		if value, ok = resourceMap[anchorKey]; ok {
			// if the key exists then validate
			// not handled for arrays
			// maps we only check if key exists
			return ValidateValueWithPattern(value, eh.pattern)
		}
		return false
	}()

	if !anchorSatisfied {
		// if the existance anchor is not satisfied then we dont process that node further
		// so we skip processing further
		return "", true, nil
	}
	// anchoredEntries++

	path, err := validateResourceElement(value, eh.pattern, originPattern, currentPath)
	if err != nil {
		return path, false, err
	}
	// if anchoredEntries == 0 {
	// 	return eh.path, fmt.Errorf("Existance anchor %s used, but no suitable entries were found", eh.anchor)
	// }
	return "", false, nil

	// anchoredEntries := 0
	// for key, element := range resourceMap {
	// 	currentPath := eh.path + key + "/"
	// 	// check for anchor condition
	// 	if !ValidateValueWithPattern(element, eh.pattern) {
	// 		// the anchor does not match so ignore
	// 		continue
	// 	}
	// 	anchoredEntries++
	// 	path, err := validateResourceElement(element, eh.pattern, originPattern, currentPath)
	// 	if err != nil {
	// 		return path, err
	// 	}
	// }
	// if anchoredEntries == 0 {
	// 	return eh.path, fmt.Errorf("Existance anchor %s used, but no suitable entries were found", eh.anchor)
	// }
	// return "", nil
}

// ValidationAnchorHandler is an interface that represents
// a family of anchor handlers for array of maps
// resourcePart must be an array of dictionaries
// patternPart must be a dictionary with anchors
type ValidationAnchorHandler interface {
	Handle(resourcePart []interface{}, patternPart map[string]interface{}, originPattern interface{}) (string, error)
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
func (navh *NoAnchorValidationHandler) Handle(resourcePart []interface{}, patternPart map[string]interface{}, originPattern interface{}) (string, error) {

	for i, resourceElement := range resourcePart {
		currentPath := navh.path + strconv.Itoa(i) + "/"

		typedResourceElement, ok := resourceElement.(map[string]interface{})
		if !ok {
			return currentPath, fmt.Errorf("Pattern and resource have different structures. Path: %s. Expected %T, found %T", currentPath, patternPart, resourceElement)
		}

		path, err := validateMap(typedResourceElement, patternPart, originPattern, currentPath)
		if err != nil {
			return path, err
		}
	}

	return "", nil
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
func (cavh *ConditionAnchorValidationHandler) Handle(resourcePart []interface{}, patternPart map[string]interface{}, originPattern interface{}) (string, error) {
	_, path, handlingResult := handleConditionCases(resourcePart, patternPart, cavh.anchor, cavh.pattern, cavh.path, originPattern)

	return path, handlingResult
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
func (eavh *ExistanceAnchorValidationHandler) Handle(resourcePart []interface{}, patternPart map[string]interface{}, originPattern interface{}) (string, error) {
	anchoredEntries, path, err := handleConditionCases(resourcePart, patternPart, eavh.anchor, eavh.pattern, eavh.path, originPattern)
	if err != nil {
		return path, err
	}
	if 0 == anchoredEntries {
		return path, fmt.Errorf("Existance anchor %s used, but no suitable entries were found", eavh.anchor)
	}

	return "", nil
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
func handleConditionCases(resourcePart []interface{}, patternPart map[string]interface{}, anchor string, pattern interface{}, path string, originPattern interface{}) (int, string, error) {
	anchoredEntries := 0

	for i, resourceElement := range resourcePart {
		currentPath := path + strconv.Itoa(i) + "/"

		typedResourceElement, ok := resourceElement.(map[string]interface{})
		if !ok {
			glog.V(4).Infof("Pattern and resource have different structures. Path: %s. Expected %T, found %T", currentPath, patternPart, resourceElement)
			return 0, currentPath, fmt.Errorf("Pattern and resource have different structures. Path: %s. Expected %T, found %T", currentPath, patternPart, resourceElement)
		}

		if !checkForAnchorCondition(anchor, pattern, typedResourceElement) {
			continue
		}

		anchoredEntries++
		path, err := validateMap(typedResourceElement, patternPart, originPattern, currentPath)
		if err != nil {
			return 0, path, err
		}
	}

	return anchoredEntries, "", nil
}
