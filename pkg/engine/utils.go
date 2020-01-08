package engine

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/golang/glog"

	"github.com/minio/minio/pkg/wildcard"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/operator"
	"github.com/nirmata/kyverno/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func findKind(kinds []string, kindGVK string) bool {
	for _, kind := range kinds {
		if kind == kindGVK {
			return true
		}
	}
	return false
}

func isStringIsReference(str string) bool {
	if len(str) < len(operator.ReferenceSign) {
		return false
	}

	return str[0] == '$' && str[1] == '(' && str[len(str)-1] == ')'
}

type resourceInfo struct {
	Resource unstructured.Unstructured
	Gvk      *metav1.GroupVersionKind
}
