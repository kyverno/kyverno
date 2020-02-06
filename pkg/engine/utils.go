package engine

import (
	"encoding/json"
	"time"

	"github.com/nirmata/kyverno/pkg/engine/rbac"

	"github.com/golang/glog"

	"github.com/minio/minio/pkg/wildcard"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/engine/variables"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

//EngineStats stores in the statistics for a single application of resource
type EngineStats struct {
	// average time required to process the policy rules on a resource
	ExecutionTime time.Duration
	// Count of rules that were applied successfully
	RulesAppliedCount int
}

func checkKind(kinds []string, resourceKind string) bool {
	for _, kind := range kinds {
		if resourceKind == kind {
			return true
		}
	}

	return false
}

func checkName(name, resourceName string) bool {
	if wildcard.Match(name, resourceName) {
		return true
	}

	return false
}

func checkNameSpace(namespaces []string, resourceNameSpace string) bool {
	for _, namespace := range namespaces {
		if resourceNameSpace == namespace {
			return true
		}
	}
	return false
}

func checkSelector(labelSelector *metav1.LabelSelector, resourceLabels map[string]string) (bool, error) {
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		glog.Error(err)
		return false, err
	}

	if selector.Matches(labels.Set(resourceLabels)) {
		return true, nil
	}

	return false, nil
}

//MatchesResourceDescription checks if the resource matches resource desription of the rule or not
func MatchesResourceDescription(resource unstructured.Unstructured, rule kyverno.Rule, admissionInfo kyverno.RequestInfo) bool {

	var condition = make(chan bool)

	go func() {
		hasPassed := rbac.MatchAdmissionInfo(rule, admissionInfo)
		if !hasPassed {
			glog.V(3).Infof("rule '%s' cannot be applied on %s/%s/%s, admission permission: %v",
				rule.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName(), admissionInfo)
		}
		condition <- hasPassed
	}()

	//
	// Match
	//
	matches := rule.MatchResources.ResourceDescription

	go func() {
		condition <- checkKind(matches.Kinds, resource.GetKind())
	}()
	go func() {
		if matches.Name != "" {
			condition <- checkName(matches.Name, resource.GetName())
		} else {
			condition <- true
		}
	}()
	go func() {
		if len(matches.Namespaces) > 0 {
			condition <- checkNameSpace(matches.Namespaces, resource.GetNamespace())
		} else {
			condition <- true
		}
	}()
	go func() {
		if matches.Selector != nil {
			hasPassed, err := checkSelector(matches.Selector, resource.GetLabels())
			if err != nil {
				condition <- false
			} else {
				condition <- hasPassed
			}
		} else {
			condition <- true
		}
	}()

	//
	// Exclude
	//
	exclude := rule.ExcludeResources.ResourceDescription

	go func() {
		if len(exclude.Kinds) > 0 {
			condition <- !checkKind(exclude.Kinds, resource.GetKind())
		} else {
			condition <- true
		}
	}()
	go func() {
		if exclude.Name != "" {
			condition <- !checkName(exclude.Name, resource.GetName())
		} else {
			condition <- true
		}
	}()
	go func() {
		if len(exclude.Namespaces) > 0 {
			condition <- !checkNameSpace(exclude.Namespaces, resource.GetNamespace())
		} else {
			condition <- true
		}
	}()
	go func() {
		if exclude.Selector != nil {
			hasPassed, err := checkSelector(exclude.Selector, resource.GetLabels())
			if err != nil {
				condition <- false
			} else {
				condition <- !hasPassed
			}
		} else {
			condition <- true
		}
	}()

	// check if any condition has failed
	var numberOfConditions = 9
	for numberOfConditions > 0 {
		select {
		case hasPassed := <-condition:
			if !hasPassed {
				return false
			}
		}
		numberOfConditions -= numberOfConditions
	}

	return true
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

// validateGeneralRuleInfoVariables validate variable subtition defined in
// - MatchResources
// - ExcludeResources
// - Conditions
func validateGeneralRuleInfoVariables(ctx context.EvalInterface, rule kyverno.Rule) string {
	var tempRule kyverno.Rule
	var tempRulePattern interface{}

	tempRule.MatchResources = rule.MatchResources
	tempRule.ExcludeResources = rule.ExcludeResources
	tempRule.Conditions = rule.Conditions

	raw, err := json.Marshal(tempRule)
	if err != nil {
		glog.Infof("failed to serilize rule info while validating variable substitution: %v", err)
		return ""
	}

	if err := json.Unmarshal(raw, &tempRulePattern); err != nil {
		glog.Infof("failed to serilize rule info while validating variable substitution: %v", err)
		return ""
	}

	return variables.ValidateVariables(ctx, tempRulePattern)
}

func newPathNotPresentRuleResponse(rname, rtype, msg string) response.RuleResponse {
	return response.RuleResponse{
		Name:           rname,
		Type:           rtype,
		Message:        msg,
		Success:        true,
		PathNotPresent: true,
	}
}
