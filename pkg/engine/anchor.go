package engine

import (
	"fmt"
	"strconv"

	"github.com/golang/glog"
)

func getAnchorsResourcesFromMap(patternMap map[string]interface{}) (map[string]interface{}, map[string]interface{}) {
	anchors := map[string]interface{}{}
	resources := map[string]interface{}{}
	for key, value := range patternMap {
		if isConditionAnchor(key) || isExistanceAnchor(key) {
			anchors[key] = value
			continue
		}
		resources[key] = value
	}

	return anchors, resources
}

type ValidationHandler interface {
	Handle(resourceMap map[string]interface{}, originPattenr interface{}) (string, error)
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

func (dh DefaultHandler) Handle(resourceMap map[string]interface{}, originPattern interface{}) (string, error) {
	currentPath := dh.path + dh.element + "/"
	if dh.pattern == "*" && resourceMap[dh.element] != nil {
		return "", nil
	} else if dh.pattern == "*" && resourceMap[dh.element] == nil {
		return dh.path, fmt.Errorf("Validation rule failed at %s, Field %s is not present", dh.path, dh.element)
	} else {
		path, err := validateResourceElement(resourceMap[dh.element], dh.pattern, originPattern, currentPath)
		if err != nil {
			return path, err
		}
	}
	return "", nil
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

func (ch ConditionAnchorHandler) Handle(resourceMap map[string]interface{}, originPattern interface{}) (string, error) {
	anchorKey := removeAnchor(ch.anchor)
	currentPath := ch.path + anchorKey + "/"
	// check if anchor is present in resource
	if value, ok := resourceMap[anchorKey]; ok {
		// validate the values of the pattern
		returnPath, err := validateResourceElement(value, ch.pattern, originPattern, currentPath)
		if err != nil {
			return returnPath, err
		}
		return "", nil

	}
	return "", nil

	// return false

	// var value interface{}
	// var currentPath string
	// var ok bool
	// // check for anchor condition
	// anchorSatisfied := func() bool {
	// 	anchorKey := removeAnchor(ch.anchor)
	// 	currentPath = ch.path + anchorKey + "/"
	// 	// check if anchor is present in resource
	// 	if value, ok = resourceMap[anchorKey]; ok {
	// 		// validate the values of the pattern
	// 		_, err := validateResourceElement(value, ch.pattern, originPattern, currentPath)
	// 		if err == nil {
	// 			return true
	// 		}
	// 		// return ValidateValueWithPattern(value, ch.pattern)
	// 	}
	// 	return false
	// }()

	// if !anchorSatisfied {
	// 	return "", nil
	// }

	// path, err := validateResourceElement(value, ch.pattern, originPattern, currentPath)
	// if err != nil {
	// 	return path, err
	// }
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
	return "", nil
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

func (eh ExistanceHandler) Handle(resourceMap map[string]interface{}, originPattern interface{}) (string, error) {
	// skip is used by existance anchor to not process further if condition is not satisfied
	anchorKey := removeAnchor(eh.anchor)
	currentPath := eh.path + anchorKey + "/"
	// check if anchor is present in resource
	if value, ok := resourceMap[anchorKey]; ok {
		// Existance anchor can only exist on resource value type of list
		switch typedResource := value.(type) {
		case []interface{}:
			typedPattern, ok := eh.pattern.([]interface{})
			if !ok {
				return currentPath, fmt.Errorf("Invalid pattern type %T: Pattern has to be of lis to compare against resource", eh.pattern)
			}
			// get the first item in the pattern array
			patternMap := typedPattern[0]
			typedPatternMap, ok := patternMap.(map[string]interface{})
			if !ok {
				return currentPath, fmt.Errorf("Invalid pattern type %T: Pattern has to be of type map to compare against items in resource", eh.pattern)
			}
			return validateExistenceListResource(typedResource, typedPatternMap, originPattern, currentPath)
		default:
			glog.Error("Invalid type: Existance ^ () anchor can be used only on list/array type resource")
			return currentPath, fmt.Errorf("Invalid resource type %T: Existance ^ () anchor can be used only on list/array type resource", value)
		}
		_, err := validateResourceElement(value, eh.pattern, originPattern, currentPath)
		if err == nil {
			// if the anchor value is the satisfied then we evaluate the next
			return "", nil
		}
		// return ValidateValueWithPattern(value, eh.pattern)
	}
	// anchoredEntries++

	// path, err := validateResourceElement(value, eh.pattern, originPattern, currentPath)
	// if err != nil {
	// 	return path, false, err
	// }
	// if anchoredEntries == 0 {
	// 	return eh.path, fmt.Errorf("Existance anchor %s used, but no suitable entries were found", eh.anchor)
	// }
	return "", nil

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

func validateExistenceListResource(resourceList []interface{}, patternMap map[string]interface{}, originPattern interface{}, path string) (string, error) {
	// the idea is atleast on the elements in the array should satisfy the pattern
	// if non satisfy then throw an error
	for i, resourceElement := range resourceList {
		currentPath := path + strconv.Itoa(i) + "/"
		_, err := validateResourceElement(resourceElement, patternMap, originPattern, currentPath)
		if err == nil {
			// condition is satisfied, dont check further
			glog.V(4).Infof("Existence check satisfied at path %s, for pattern %v", currentPath, patternMap)
			return "", nil
		}
	}
	// none of the existance checks worked, so thats a failure sceanario
	return path, fmt.Errorf("Existence anchor validation failed at path %s", path)
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
