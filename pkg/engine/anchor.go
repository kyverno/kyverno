package engine

import (
	"fmt"
	"strconv"

	"github.com/golang/glog"
)

//ValidationHandler for element processes
type ValidationHandler interface {
	Handle(resourceMap map[string]interface{}, originPattenr interface{}) (string, error)
}

//CreateElementHandler factory to process elements
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

//NewDefaultHandler returns handler for non anchor elements
func NewDefaultHandler(element string, pattern interface{}, path string) ValidationHandler {
	return DefaultHandler{
		element: element,
		pattern: pattern,
		path:    path,
	}
}

//DefaultHandler provides handler for non anchor element
type DefaultHandler struct {
	element string
	pattern interface{}
	path    string
}

//Handle process non anchor element
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

//NewConditionAnchorHandler returns an instance of condition acnhor handler
func NewConditionAnchorHandler(anchor string, pattern interface{}, path string) ValidationHandler {
	return ConditionAnchorHandler{
		anchor:  anchor,
		pattern: pattern,
		path:    path,
	}
}

//ConditionAnchorHandler provides handler for condition anchor
type ConditionAnchorHandler struct {
	anchor  string
	pattern interface{}
	path    string
}

//Handle processed condition anchor
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
}

//NewExistanceHandler returns existence handler
func NewExistanceHandler(anchor string, pattern interface{}, path string) ValidationHandler {
	return ExistanceHandler{
		anchor:  anchor,
		pattern: pattern,
		path:    path,
	}
}

//ExistanceHandler provides handlers to process exitence anchor handler
type ExistanceHandler struct {
	anchor  string
	pattern interface{}
	path    string
}

//Handle processes the existence anchor handler
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
	}
	return "", nil
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
