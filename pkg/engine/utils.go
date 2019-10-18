package engine

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"

	"github.com/minio/minio/pkg/wildcard"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	"github.com/nirmata/kyverno/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

//EngineStats stores in the statistics for a single application of resource
type EngineStats struct {
	// average time required to process the policy rules on a resource
	ExecutionTime time.Duration
	// Count of rules that were applied succesfully
	RulesAppliedCount int
}

//MatchesResourceDescription checks if the resource matches resource desription of the rule or not
func MatchesResourceDescription(resource unstructured.Unstructured, rule kyverno.Rule) bool {
	matches := rule.MatchResources.ResourceDescription
	exclude := rule.ExcludeResources.ResourceDescription

	if !findKind(matches.Kinds, resource.GetKind()) {
		return false
	}

	name := resource.GetName()

	namespace := resource.GetNamespace()

	if matches.Name != "" {
		// Matches
		if !wildcard.Match(matches.Name, name) {
			return false
		}
	}

	// Matches
	// check if the resource namespace is defined in the list of namespace pattern
	if len(matches.Namespaces) > 0 && !utils.ContainsNamepace(matches.Namespaces, namespace) {
		return false
	}

	// Matches
	if matches.Selector != nil {
		selector, err := metav1.LabelSelectorAsSelector(matches.Selector)
		if err != nil {
			glog.Error(err)
			return false
		}
		if !selector.Matches(labels.Set(resource.GetLabels())) {
			return false
		}
	}

	excludeName := func(name string) Condition {
		if exclude.Name == "" {
			return NotEvaluate
		}
		if wildcard.Match(exclude.Name, name) {
			return Skip
		}
		return Process
	}

	excludeNamespace := func(namespace string) Condition {
		if len(exclude.Namespaces) == 0 {
			return NotEvaluate
		}
		if utils.ContainsNamepace(exclude.Namespaces, namespace) {
			return Skip
		}
		return Process
	}

	excludeSelector := func(labelsMap map[string]string) Condition {
		if exclude.Selector == nil {
			return NotEvaluate
		}
		selector, err := metav1.LabelSelectorAsSelector(exclude.Selector)
		// if the label selector is incorrect, should be fail or
		if err != nil {
			glog.Error(err)
			return Skip
		}
		if selector.Matches(labels.Set(labelsMap)) {
			return Skip
		}
		return Process
	}

	excludeKind := func(kind string) Condition {
		if len(exclude.Kinds) == 0 {
			return NotEvaluate
		}

		if findKind(exclude.Kinds, kind) {
			return Skip
		}

		return Process
	}

	// 0 -> dont check
	// 1 -> is not to be exclude
	// 2 -> to be exclude
	excludeEval := []Condition{}

	if ret := excludeName(resource.GetName()); ret != NotEvaluate {
		excludeEval = append(excludeEval, ret)
	}
	if ret := excludeNamespace(resource.GetNamespace()); ret != NotEvaluate {
		excludeEval = append(excludeEval, ret)
	}
	if ret := excludeSelector(resource.GetLabels()); ret != NotEvaluate {
		excludeEval = append(excludeEval, ret)
	}
	if ret := excludeKind(resource.GetKind()); ret != NotEvaluate {
		excludeEval = append(excludeEval, ret)
	}
	// Filtered NotEvaluate

	if len(excludeEval) == 0 {
		// nothing to exclude
		return true
	}
	return func() bool {
		for _, ret := range excludeEval {
			if ret == Process {
				return true
			}
		}
		return false
	}()
}

type Condition int

const (
	NotEvaluate Condition = 0
	Process     Condition = 1
	Skip        Condition = 2
)

// ParseResourceInfoFromObject get kind/namepace/name from resource
func ParseResourceInfoFromObject(rawResource []byte) string {

	kind := ParseKindFromObject(rawResource)
	namespace := ParseNamespaceFromObject(rawResource)
	name := ParseNameFromObject(rawResource)
	return strings.Join([]string{kind, namespace, name}, "/")
}

//ParseKindFromObject get kind from resource
func ParseKindFromObject(bytes []byte) string {
	var objectJSON map[string]interface{}
	json.Unmarshal(bytes, &objectJSON)

	return objectJSON["kind"].(string)
}

//ParseNameFromObject extracts resource name from JSON obj
func ParseNameFromObject(bytes []byte) string {
	var objectJSON map[string]interface{}
	json.Unmarshal(bytes, &objectJSON)
	meta, ok := objectJSON["metadata"]
	if !ok {
		return ""
	}

	metaMap, ok := meta.(map[string]interface{})
	if !ok {
		return ""
	}
	if name, ok := metaMap["name"].(string); ok {
		return name
	}
	return ""
}

// ParseNamespaceFromObject extracts the namespace from the JSON obj
func ParseNamespaceFromObject(bytes []byte) string {
	var objectJSON map[string]interface{}
	json.Unmarshal(bytes, &objectJSON)
	meta, ok := objectJSON["metadata"]
	if !ok {
		return ""
	}
	metaMap, ok := meta.(map[string]interface{})
	if !ok {
		return ""
	}

	if name, ok := metaMap["namespace"].(string); ok {
		return name
	}

	return ""
}

func getAnchorsFromMap(anchorsMap map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range anchorsMap {
		if isConditionAnchor(key) || isExistanceAnchor(key) {
			result[key] = value
		}
	}

	return result
}

func getElementsFromMap(anchorsMap map[string]interface{}) (map[string]interface{}, map[string]interface{}) {
	anchors := make(map[string]interface{})
	elementsWithoutanchor := make(map[string]interface{})
	for key, value := range anchorsMap {
		if isConditionAnchor(key) || isExistanceAnchor(key) {
			anchors[key] = value
		} else if !isAddingAnchor(key) {
			elementsWithoutanchor[key] = value
		}
	}

	return anchors, elementsWithoutanchor
}

func getAnchorFromMap(anchorsMap map[string]interface{}) (string, interface{}) {
	for key, value := range anchorsMap {
		if isConditionAnchor(key) || isExistanceAnchor(key) {
			return key, value
		}
	}

	return "", nil
}

func findKind(kinds []string, kindGVK string) bool {
	for _, kind := range kinds {
		if kind == kindGVK {
			return true
		}
	}
	return false
}

func isConditionAnchor(str string) bool {
	if len(str) < 2 {
		return false
	}

	return (str[0] == '(' && str[len(str)-1] == ')')
}

func getRawKeyIfWrappedWithAttributes(str string) string {
	if len(str) < 2 {
		return str
	}

	if str[0] == '(' && str[len(str)-1] == ')' {
		return str[1 : len(str)-1]
	} else if (str[0] == '$' || str[0] == '^' || str[0] == '+' || str[0] == '=') && (str[1] == '(' && str[len(str)-1] == ')') {
		return str[2 : len(str)-1]
	} else {
		return str
	}
}

func isStringIsReference(str string) bool {
	if len(str) < len(referenceSign) {
		return false
	}

	return str[0] == '$' && str[1] == '(' && str[len(str)-1] == ')'
}

func isExistanceAnchor(str string) bool {
	left := "^("
	right := ")"

	if len(str) < len(left)+len(right) {
		return false
	}

	return (str[:len(left)] == left && str[len(str)-len(right):] == right)
}

func isEqualityAnchor(str string) bool {
	left := "=("
	right := ")"
	if len(str) < len(left)+len(right) {
		return false
	}
	//TODO: trim spaces ?
	return (str[:len(left)] == left && str[len(str)-len(right):] == right)
}

func isNegationAnchor(str string) bool {
	left := "X("
	right := ")"
	if len(str) < len(left)+len(right) {
		return false
	}
	//TODO: trim spaces ?
	return (str[:len(left)] == left && str[len(str)-len(right):] == right)
}

func isAddingAnchor(key string) bool {
	const left = "+("
	const right = ")"

	if len(key) < len(left)+len(right) {
		return false
	}

	return left == key[:len(left)] && right == key[len(key)-len(right):]
}

// Checks if array object matches anchors. If not - skip - return true
func skipArrayObject(object, anchors map[string]interface{}) bool {
	for key, pattern := range anchors {
		key = key[1 : len(key)-1]

		value, ok := object[key]
		if !ok {
			return true
		}

		if !ValidateValueWithPattern(value, pattern) {
			return true
		}
	}

	return false
}

// removeAnchor remove special characters around anchored key
func removeAnchor(key string) string {
	if isConditionAnchor(key) {
		return key[1 : len(key)-1]
	}

	if isExistanceAnchor(key) || isAddingAnchor(key) || isEqualityAnchor(key) || isNegationAnchor(key) {
		return key[2 : len(key)-1]
	}

	return key
}

// convertToFloat converts string and any other value to float64
func convertToFloat(value interface{}) (float64, error) {
	switch typed := value.(type) {
	case string:
		var err error
		floatValue, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return 0, err
		}

		return floatValue, nil
	case float64:
		return typed, nil
	case int64:
		return float64(typed), nil
	case int:
		return float64(typed), nil
	default:
		return 0, fmt.Errorf("Could not convert %T to float64", value)
	}
}

type resourceInfo struct {
	Resource unstructured.Unstructured
	Gvk      *metav1.GroupVersionKind
}

func ConvertToUnstructured(data []byte) (*unstructured.Unstructured, error) {
	resource := &unstructured.Unstructured{}
	err := resource.UnmarshalJSON(data)
	if err != nil {
		glog.V(4).Infof("failed to unmarshall resource: %v", err)
		return nil, err
	}
	return resource, nil
}

type RuleType int

const (
	Mutation RuleType = iota
	Validation
	Generation
	All
)

func (ri RuleType) String() string {
	return [...]string{
		"Mutation",
		"Validation",
		"Generation",
		"All",
	}[ri]
}
