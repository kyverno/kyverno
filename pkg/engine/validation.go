package engine

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	"github.com/nirmata/kyverno/pkg/engine/anchor"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

//Validate applies validation rules from policy on the resource
func Validate(policy kyverno.ClusterPolicy, resource unstructured.Unstructured) (response EngineResponse) {
	startTime := time.Now()
	// policy information
	func() {
		// set policy information
		response.PolicyResponse.Policy = policy.Name
		// resource details
		response.PolicyResponse.Resource.Name = resource.GetName()
		response.PolicyResponse.Resource.Namespace = resource.GetNamespace()
		response.PolicyResponse.Resource.Kind = resource.GetKind()
		response.PolicyResponse.Resource.APIVersion = resource.GetAPIVersion()
		response.PolicyResponse.ValidationFailureAction = policy.Spec.ValidationFailureAction
	}()

	glog.V(4).Infof("started applying validation rules of policy %q (%v)", policy.Name, startTime)
	defer func() {
		response.PolicyResponse.ProcessingTime = time.Since(startTime)
		glog.V(4).Infof("Finished applying validation rules policy %v (%v)", policy.Name, response.PolicyResponse.ProcessingTime)
		glog.V(4).Infof("Validation Rules appplied succesfully count %v for policy %q", response.PolicyResponse.RulesAppliedCount, policy.Name)
	}()
	incrementAppliedRuleCount := func() {
		// rules applied succesfully count
		response.PolicyResponse.RulesAppliedCount++
	}

	for _, rule := range policy.Spec.Rules {
		if !rule.HasValidate() {
			continue
		}
		// check if the resource satisfies the filter conditions defined in the rule
		// TODO: this needs to be extracted, to filter the resource so that we can avoid passing resources that
		// dont statisfy a policy rule resource description
		ok := MatchesResourceDescription(resource, rule)
		if !ok {
			glog.V(4).Infof("resource %s/%s does not satisfy the resource description for the rule ", resource.GetNamespace(), resource.GetName())
			continue
		}
		if rule.Validation.Pattern != nil || rule.Validation.AnyPattern != nil {
			ruleResponse := validatePatterns(resource, rule)
			incrementAppliedRuleCount()
			response.PolicyResponse.Rules = append(response.PolicyResponse.Rules, ruleResponse)
		}
	}

	// set PatchedResource with orgin resource if empty
	// in order to create policy violation
	if reflect.DeepEqual(response.PatchedResource, unstructured.Unstructured{}) {
		response.PatchedResource = resource
	}

	return response
}

// validatePatterns validate pattern and anyPattern
func validatePatterns(resource unstructured.Unstructured, rule kyverno.Rule) (response RuleResponse) {
	startTime := time.Now()
	glog.V(4).Infof("started applying validation rule %q (%v)", rule.Name, startTime)
	response.Name = rule.Name
	response.Type = Validation.String()
	defer func() {
		response.RuleStats.ProcessingTime = time.Since(startTime)
		glog.V(4).Infof("finished applying validation rule %q (%v)", response.Name, response.RuleStats.ProcessingTime)
	}()

	// either pattern or anyPattern can be specified in Validation rule
	if rule.Validation.Pattern != nil {
		path, err := validateResourceWithPattern(resource.Object, rule.Validation.Pattern)
		if err != nil {
			// rule application failed
			glog.V(4).Infof("Validation rule '%s' failed at '%s' for resource %s/%s/%s. %s: %v", rule.Name, path, resource.GetKind(), resource.GetNamespace(), resource.GetName(), rule.Validation.Message, err)
			response.Success = false
			response.Message = fmt.Sprintf("Validation error: %s\nValidation rule '%s' failed at path '%s'.",
				rule.Validation.Message, rule.Name, path)
			return response
		}
		// rule application succesful
		glog.V(4).Infof("rule %s pattern validated succesfully on resource %s/%s/%s", rule.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName())
		response.Success = true
		response.Message = fmt.Sprintf("Validation rule '%s' succeeded.", rule.Name)
		return response
	}

	// using anyPattern we can define multiple patterns and only one of them has to be succesfully validated
	if rule.Validation.AnyPattern != nil {
		var errs []error
		var failedPaths []string
		for index, pattern := range rule.Validation.AnyPattern {
			path, err := validateResourceWithPattern(resource.Object, pattern)
			if err == nil {
				// this pattern was succesfully validated
				glog.V(4).Infof("anyPattern %v succesfully validated on resource %s/%s/%s", pattern, resource.GetKind(), resource.GetNamespace(), resource.GetName())
				response.Success = true
				response.Message = fmt.Sprintf("Validation rule '%s' anyPattern[%d] succeeded.", rule.Name, index)
				return response
			}
			if err != nil {
				glog.V(4).Infof("Validation error: %s\nValidation rule %s anyPattern[%d] failed at path %s for %s/%s/%s",
					rule.Validation.Message, rule.Name, index, path, resource.GetKind(), resource.GetNamespace(), resource.GetName())
				errs = append(errs, err)
				failedPaths = append(failedPaths, path)
			}
		}
		// If none of the anyPatterns are validated
		if len(errs) > 0 {
			glog.V(4).Infof("none of anyPattern were processed: %v", errs)
			response.Success = false
			var errorStr []string
			for index, err := range errs {
				glog.V(4).Infof("anyPattern[%d] failed at path %s: %v", index, failedPaths[index], err)
				str := fmt.Sprintf("Validation rule %s anyPattern[%d] failed at path %s.", rule.Name, index, failedPaths[index])
				errorStr = append(errorStr, str)
			}
			response.Message = fmt.Sprintf("Validation error: %s\n%s", rule.Validation.Message, strings.Join(errorStr, "\n"))

			return response
		}
	}
	return RuleResponse{}
}

// validateResourceWithPattern is a start of element-by-element validation process
// It assumes that validation is started from root, so "/" is passed
//TODO: for failure, we return the path at which it failed along with error
func validateResourceWithPattern(resource, pattern interface{}) (string, error) {
	return validateResourceElement(resource, pattern, pattern, "/")
}

// validateResourceElement detects the element type (map, array, nil, string, int, bool, float)
// and calls corresponding handler
// Pattern tree and resource tree can have different structure. In this case validation fails
func validateResourceElement(resourceElement, patternElement, originPattern interface{}, path string) (string, error) {
	var err error
	switch typedPatternElement := patternElement.(type) {
	// map
	case map[string]interface{}:
		typedResourceElement, ok := resourceElement.(map[string]interface{})
		if !ok {
			glog.V(4).Infof("Pattern and resource have different structures. Path: %s. Expected %T, found %T", path, patternElement, resourceElement)
			return path, fmt.Errorf("Pattern and resource have different structures. Path: %s. Expected %T, found %T", path, patternElement, resourceElement)
		}

		return validateMap(typedResourceElement, typedPatternElement, originPattern, path)
	// array
	case []interface{}:
		typedResourceElement, ok := resourceElement.([]interface{})
		if !ok {
			glog.V(4).Infof("Pattern and resource have different structures. Path: %s. Expected %T, found %T", path, patternElement, resourceElement)
			return path, fmt.Errorf("Validation rule Failed at path %s, resource does not satisfy the expected overlay pattern", path)
		}

		return validateArray(typedResourceElement, typedPatternElement, originPattern, path)
	// elementary values
	case string, float64, int, int64, bool, nil:
		/*Analyze pattern */
		if checkedPattern := reflect.ValueOf(patternElement); checkedPattern.Kind() == reflect.String {
			if isStringIsReference(checkedPattern.String()) { //check for $ anchor
				patternElement, err = actualizePattern(originPattern, checkedPattern.String(), path)
				if err != nil {
					return path, err
				}
			}
		}
		if !ValidateValueWithPattern(resourceElement, patternElement) {
			return path, fmt.Errorf("Validation rule failed at '%s' to validate value %v with pattern %v", path, resourceElement, patternElement)
		}

	default:
		glog.V(4).Infof("Pattern contains unknown type %T. Path: %s", patternElement, path)
		return path, fmt.Errorf("Validation rule failed at '%s', pattern contains unknown type", path)
	}
	return "", nil
}

// If validateResourceElement detects map element inside resource and pattern trees, it goes to validateMap
// For each element of the map we must detect the type again, so we pass these elements to validateResourceElement
func validateMap(resourceMap, patternMap map[string]interface{}, origPattern interface{}, path string) (string, error) {
	// check if there is anchor in pattern
	// Phase 1 : Evaluate all the anchors
	// Phase 2 : Evaluate non-anchors
	anchors, resources := getAnchorsResourcesFromMap(patternMap)

	// Evaluate anchors
	for key, patternElement := range anchors {
		// get handler for each pattern in the pattern
		// - Conditional
		// - Existance
		// - Equality
		handler := CreateElementHandler(key, patternElement, path)
		handlerPath, err := handler.Handle(resourceMap, origPattern)
		// if there are resource values at same level, then anchor acts as conditional instead of a strict check
		// but if there are non then its a if then check
		if err != nil {
			// If Conditional anchor fails then we dont process the resources
			if anchor.IsConditionAnchor(key) {
				glog.V(4).Infof("condition anchor did not satisfy, wont process the resources: %s", err)
				return "", nil
			}
			return handlerPath, err
		}
	}
	// Evaluate resources
	for key, resourceElement := range resources {
		// get handler for resources in the pattern
		handler := CreateElementHandler(key, resourceElement, path)
		handlerPath, err := handler.Handle(resourceMap, origPattern)
		if err != nil {
			return handlerPath, err
		}
	}
	return "", nil
}

func validateArray(resourceArray, patternArray []interface{}, originPattern interface{}, path string) (string, error) {

	if 0 == len(patternArray) {
		return path, fmt.Errorf("Pattern Array empty")
	}

	switch typedPatternElement := patternArray[0].(type) {
	case map[string]interface{}:
		// This is special case, because maps in arrays can have anchors that must be
		// processed with the special way affecting the entire array
		path, err := validateArrayOfMaps(resourceArray, typedPatternElement, originPattern, path)
		if err != nil {
			return path, err
		}
	default:
		// In all other cases - detect type and handle each array element with validateResourceElement
		for i, patternElement := range patternArray {
			currentPath := path + strconv.Itoa(i) + "/"
			path, err := validateResourceElement(resourceArray[i], patternElement, originPattern, currentPath)
			if err != nil {
				return path, err
			}
		}
	}

	return "", nil
}

func actualizePattern(origPattern interface{}, referencePattern, absolutePath string) (interface{}, error) {
	var foundValue interface{}

	referencePattern = strings.Trim(referencePattern, "$()")

	operator := getOperatorFromStringPattern(referencePattern)
	referencePattern = referencePattern[len(operator):]

	if len(referencePattern) == 0 {
		return nil, errors.New("Expected path. Found empty reference")
	}

	actualPath := FormAbsolutePath(referencePattern, absolutePath)

	valFromReference, err := getValueFromReference(origPattern, actualPath)
	if err != nil {
		return err, nil
	}
	//TODO validate this
	if operator == Equal { //if operator does not exist return raw value
		return valFromReference, nil
	}

	foundValue, err = valFromReferenceToString(valFromReference, string(operator))
	if err != nil {
		return "", err
	}
	return string(operator) + foundValue.(string), nil
}

//Parse value to string
func valFromReferenceToString(value interface{}, operator string) (string, error) {

	switch typed := value.(type) {
	case string:
		return typed, nil
	case int, int64:
		return fmt.Sprintf("%d", value), nil
	case float64:
		return fmt.Sprintf("%f", value), nil
	default:
		return "", fmt.Errorf("Incorrect expression. Operator %s does not match with value: %v", operator, value)
	}
}

//FormAbsolutePath returns absolute path
func FormAbsolutePath(referencePath, absolutePath string) string {
	if filepath.IsAbs(referencePath) {
		return referencePath
	}

	return filepath.Join(absolutePath, referencePath)
}

//Prepares original pattern, path to value, and call traverse function
func getValueFromReference(origPattern interface{}, reference string) (interface{}, error) {
	originalPatternMap := origPattern.(map[string]interface{})
	reference = reference[1:len(reference)]
	statements := strings.Split(reference, "/")

	return getValueFromPattern(originalPatternMap, statements, 0)
}

func getValueFromPattern(patternMap map[string]interface{}, keys []string, currentKeyIndex int) (interface{}, error) {

	for key, pattern := range patternMap {
		rawKey := getRawKeyIfWrappedWithAttributes(key)

		if rawKey == keys[len(keys)-1] && currentKeyIndex == len(keys)-1 {
			return pattern, nil
		} else if rawKey != keys[currentKeyIndex] && currentKeyIndex != len(keys)-1 {
			continue
		}

		switch typedPattern := pattern.(type) {
		case []interface{}:
			if keys[currentKeyIndex] == rawKey {
				for i, value := range typedPattern {
					resourceMap, ok := value.(map[string]interface{})
					if !ok {
						glog.V(4).Infof("Pattern and resource have different structures. Expected %T, found %T", pattern, value)
						return nil, fmt.Errorf("Validation rule failed, resource does not have expected pattern %v", patternMap)
					}
					if keys[currentKeyIndex+1] == strconv.Itoa(i) {
						return getValueFromPattern(resourceMap, keys, currentKeyIndex+2)
					}
					return nil, errors.New("Reference to non-existent place in the document")
				}
			}
			return nil, errors.New("Reference to non-existent place in the document")
		case map[string]interface{}:
			if keys[currentKeyIndex] == rawKey {
				return getValueFromPattern(typedPattern, keys, currentKeyIndex+1)
			}
			return nil, errors.New("Reference to non-existent place in the document")
		case string, float64, int, int64, bool, nil:
			continue
		}
	}

	path := ""

	/*for i := len(keys) - 1; i >= 0; i-- {
		path = keys[i] + path + "/"
	}*/
	for _, elem := range keys {
		path = "/" + elem + path
	}
	return nil, fmt.Errorf("No value found for specified reference: %s", path)
}

// validateArrayOfMaps gets anchors from pattern array map element, applies anchors logic
// and then validates each map due to the pattern
func validateArrayOfMaps(resourceMapArray []interface{}, patternMap map[string]interface{}, originPattern interface{}, path string) (string, error) {
	for i, resourceElement := range resourceMapArray {
		// check the types of resource element
		// expect it to be map, but can be anything ?:(
		currentPath := path + strconv.Itoa(i) + "/"
		returnpath, err := validateResourceElement(resourceElement, patternMap, originPattern, currentPath)
		if err != nil {
			return returnpath, err
		}
	}
	return "", nil
}
