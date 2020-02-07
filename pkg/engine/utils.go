package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
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
	return wildcard.Match(name, resourceName)
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

func doesResourceMatchConditionBlock(conditionBlock kyverno.ResourceDescription, resource unstructured.Unstructured) []error {
	var wg sync.WaitGroup
	wg.Add(4)
	var errs = make(chan error, 4)
	go func() {
		if len(conditionBlock.Kinds) > 0 {
			if !checkKind(conditionBlock.Kinds, resource.GetKind()) {
				errs <- fmt.Errorf("resource kind does not match conditionBlock")
			}
		}
		wg.Done()
	}()
	go func() {
		if conditionBlock.Name != "" {
			if !checkName(conditionBlock.Name, resource.GetName()) {
				errs <- fmt.Errorf("resource name does not match conditionBlock")
			}
		}
		wg.Done()
	}()
	go func() {
		if len(conditionBlock.Namespaces) > 0 {
			if !checkNameSpace(conditionBlock.Namespaces, resource.GetNamespace()) {
				errs <- fmt.Errorf("resource namespace does not match conditionBlock")
			}
		}
		wg.Done()
	}()
	go func() {
		if conditionBlock.Selector != nil {
			hasPassed, err := checkSelector(conditionBlock.Selector, resource.GetLabels())
			if err != nil {
				errs <- fmt.Errorf("could not parse selector block of the policy in conditionBlock: %v", err)
			} else {
				if !hasPassed {
					errs <- fmt.Errorf("resource does not match selector of given conditionBlock")
				}
			}
		}
		wg.Done()
	}()
	wg.Wait()
	close(errs)

	var errsIfAny []error
	for err := range errs {
		errsIfAny = append(errsIfAny, err)
	}

	return errsIfAny
}

//MatchesResourceDescription checks if the resource matches resource description of the rule or not
func MatchesResourceDescription(resource unstructured.Unstructured, rule kyverno.Rule, admissionInfo kyverno.RequestInfo) error {
	var errs = make(chan error, 6)
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		if !rbac.MatchAdmissionInfo(rule, admissionInfo) {
			errs <- fmt.Errorf("rule '%s' cannot be applied on %s/%s/%s, admission permission: %v",
				rule.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName(), admissionInfo)
		}
		wg.Done()
	}()

	// checking if resource matches the rule
	go func() {
		if !reflect.DeepEqual(rule.MatchResources.ResourceDescription, kyverno.ResourceDescription{}) {
			matchErrs := doesResourceMatchConditionBlock(rule.MatchResources.ResourceDescription, resource)
			for _, matchErr := range matchErrs {
				errs <- matchErr
			}
		} else {
			errs <- fmt.Errorf("match block in rule cannot be empty")
		}
		wg.Done()
	}()

	// checking if resource has been excluded
	go func() {
		if !reflect.DeepEqual(rule.ExcludeResources.ResourceDescription, kyverno.ResourceDescription{}) {
			excludeErrs := doesResourceMatchConditionBlock(rule.ExcludeResources.ResourceDescription, resource)
			if excludeErrs == nil {
				errs <- fmt.Errorf("resource has been excluded since it matches the exclude block")
			}
		}
		wg.Done()
	}()

	wg.Wait()
	close(errs)

	var reasonsForFailure []error
	for err := range errs {
		reasonsForFailure = append(reasonsForFailure, err)
	}

	// creating final error
	var errorMessage = "rule has failed to match resource for the following reasons:"
	for i, reasonForFailure := range reasonsForFailure {
		if reasonForFailure != nil {
			errorMessage += "\n" + fmt.Sprint(i+1) + ". " + reasonForFailure.Error()
		}
	}

	if len(reasonsForFailure) > 0 {
		return errors.New(errorMessage)
	}

	return nil
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
