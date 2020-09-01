package anchor

import (
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	commonAnchors "github.com/nirmata/kyverno/pkg/engine/anchor/common"
	"github.com/nirmata/kyverno/pkg/engine/common"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//ValidationHandler for element processes
type ValidationHandler interface {
	Handle(handler resourceElementHandler, resourceMap map[string]interface{}, originPattern interface{}, ac *common.AnchorKey) (string, error)
}

type resourceElementHandler = func(log logr.Logger, resourceElement, patternElement, originPattern interface{}, path string, ac *common.AnchorKey) (string, error)

//CreateElementHandler factory to process elements
func CreateElementHandler(element string, pattern interface{}, path string) ValidationHandler {
	switch {
	case commonAnchors.IsConditionAnchor(element):
		return NewConditionAnchorHandler(element, pattern, path)
	case commonAnchors.IsExistenceAnchor(element):
		return NewExistenceHandler(element, pattern, path)
	case commonAnchors.IsEqualityAnchor(element):
		return NewEqualityHandler(element, pattern, path)
	case commonAnchors.IsNegationAnchor(element):
		return NewNegationHandler(element, pattern, path)
	default:
		return NewDefaultHandler(element, pattern, path)
	}
}

//NewNegationHandler returns instance of negation handler
func NewNegationHandler(anchor string, pattern interface{}, path string) ValidationHandler {
	return NegationHandler{
		anchor:  anchor,
		pattern: pattern,
		path:    path,
	}
}

//NegationHandler provides handler for check if the tag in anchor is not defined
type NegationHandler struct {
	anchor  string
	pattern interface{}
	path    string
}

//Handle process negation handler
func (nh NegationHandler) Handle(handler resourceElementHandler, resourceMap map[string]interface{}, originPattern interface{}, ac *common.AnchorKey) (string, error) {
	anchorKey := commonAnchors.RemoveAnchor(nh.anchor)
	currentPath := nh.path + anchorKey + "/"
	// if anchor is present in the resource then fail
	if _, ok := resourceMap[anchorKey]; ok {
		// no need to process elements in value as key cannot be present in resource
		return currentPath, fmt.Errorf("Validation rule failed at %s, field %s is disallowed", currentPath, anchorKey)
	}
	// key is not defined in the resource
	return "", nil
}

//NewEqualityHandler returens instance of equality handler
func NewEqualityHandler(anchor string, pattern interface{}, path string) ValidationHandler {
	return EqualityHandler{
		anchor:  anchor,
		pattern: pattern,
		path:    path,
	}
}

//EqualityHandler provides handler for non anchor element
type EqualityHandler struct {
	anchor  string
	pattern interface{}
	path    string
}

//Handle processed condition anchor
func (eh EqualityHandler) Handle(handler resourceElementHandler, resourceMap map[string]interface{}, originPattern interface{}, ac *common.AnchorKey) (string, error) {
	anchorKey := commonAnchors.RemoveAnchor(eh.anchor)
	currentPath := eh.path + anchorKey + "/"
	// check if anchor is present in resource
	if value, ok := resourceMap[anchorKey]; ok {
		// validate the values of the pattern
		returnPath, err := handler(log.Log, value, eh.pattern, originPattern, currentPath, ac)
		if err != nil {
			return returnPath, err
		}
		return "", nil
	}
	return "", nil
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
func (dh DefaultHandler) Handle(handler resourceElementHandler, resourceMap map[string]interface{}, originPattern interface{}, ac *common.AnchorKey) (string, error) {
	currentPath := dh.path + dh.element + "/"
	if dh.pattern == "*" && resourceMap[dh.element] != nil {
		return "", nil
	} else if dh.pattern == "*" && resourceMap[dh.element] == nil {
		return dh.path, fmt.Errorf("Validation rule failed at %s, Field %s is not present", dh.path, dh.element)
	} else {
		path, err := handler(log.Log, resourceMap[dh.element], dh.pattern, originPattern, currentPath, ac)
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
func (ch ConditionAnchorHandler) Handle(handler resourceElementHandler, resourceMap map[string]interface{}, originPattern interface{}, ac *common.AnchorKey) (string, error) {
	anchorKey := commonAnchors.RemoveAnchor(ch.anchor)
	currentPath := ch.path + anchorKey + "/"
	// check if anchor is present in resource
	if value, ok := resourceMap[anchorKey]; ok {
		// validate the values of the pattern
		returnPath, err := handler(log.Log, value, ch.pattern, originPattern, currentPath, ac)
		if err != nil {
			return returnPath, err
		}
		return "", nil

	}
	return "", nil
}

//NewExistenceHandler returns existence handler
func NewExistenceHandler(anchor string, pattern interface{}, path string) ValidationHandler {
	return ExistenceHandler{
		anchor:  anchor,
		pattern: pattern,
		path:    path,
	}
}

//ExistenceHandler provides handlers to process exitence anchor handler
type ExistenceHandler struct {
	anchor  string
	pattern interface{}
	path    string
}

//Handle processes the existence anchor handler
func (eh ExistenceHandler) Handle(handler resourceElementHandler, resourceMap map[string]interface{}, originPattern interface{}, ac *common.AnchorKey) (string, error) {
	// skip is used by existence anchor to not process further if condition is not satisfied
	anchorKey := commonAnchors.RemoveAnchor(eh.anchor)
	currentPath := eh.path + anchorKey + "/"
	// check if anchor is present in resource
	if value, ok := resourceMap[anchorKey]; ok {
		// Existence anchor can only exist on resource value type of list
		switch typedResource := value.(type) {
		case []interface{}:
			typedPattern, ok := eh.pattern.([]interface{})
			if !ok {
				return currentPath, fmt.Errorf("Invalid pattern type %T: Pattern has to be of list to compare against resource", eh.pattern)
			}
			// get the first item in the pattern array
			patternMap := typedPattern[0]
			typedPatternMap, ok := patternMap.(map[string]interface{})
			if !ok {
				return currentPath, fmt.Errorf("Invalid pattern type %T: Pattern has to be of type map to compare against items in resource", eh.pattern)
			}
			return validateExistenceListResource(handler, typedResource, typedPatternMap, originPattern, currentPath, ac)
		default:
			return currentPath, fmt.Errorf("Invalid resource type %T: Existence ^ () anchor can be used only on list/array type resource", value)
		}
	}
	return "", nil
}

func validateExistenceListResource(handler resourceElementHandler, resourceList []interface{}, patternMap map[string]interface{}, originPattern interface{}, path string, ac *common.AnchorKey) (string, error) {
	// the idea is atleast on the elements in the array should satisfy the pattern
	// if non satisfy then throw an error
	for i, resourceElement := range resourceList {
		currentPath := path + strconv.Itoa(i) + "/"
		_, err := handler(log.Log, resourceElement, patternMap, originPattern, currentPath, ac)
		if err == nil {
			// condition is satisfied, dont check further
			return "", nil
		}
	}
	// none of the existence checks worked, so thats a failure sceanario
	return path, fmt.Errorf("Existence anchor validation failed at path %s", path)
}

//GetAnchorsResourcesFromMap returns map of anchors
func GetAnchorsResourcesFromMap(patternMap map[string]interface{}) (map[string]interface{}, map[string]interface{}) {
	anchors := map[string]interface{}{}
	resources := map[string]interface{}{}
	for key, value := range patternMap {
		if commonAnchors.IsConditionAnchor(key) || commonAnchors.IsExistenceAnchor(key) || commonAnchors.IsEqualityAnchor(key) || commonAnchors.IsNegationAnchor(key) {
			anchors[key] = value
			continue
		}
		resources[key] = value
	}

	return anchors, resources
}
