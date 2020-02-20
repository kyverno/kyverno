package engine

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/nirmata/kyverno/pkg/utils"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/golang/glog"

	"github.com/minio/minio/pkg/wildcard"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
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

func doesResourceMatchConditionBlock(conditionBlock kyverno.ResourceDescription, userInfo kyverno.UserInfo, admissionInfo kyverno.RequestInfo, resource unstructured.Unstructured) []error {
	var errs []error
	if len(conditionBlock.Kinds) > 0 {
		if !checkKind(conditionBlock.Kinds, resource.GetKind()) {
			errs = append(errs, fmt.Errorf("resource kind does not match conditionBlock"))
		}
	}
	if conditionBlock.Name != "" {
		if !checkName(conditionBlock.Name, resource.GetName()) {
			errs = append(errs, fmt.Errorf("resource name does not match conditionBlock"))
		}
	}
	if len(conditionBlock.Namespaces) > 0 {
		if !checkNameSpace(conditionBlock.Namespaces, resource.GetNamespace()) {
			errs = append(errs, fmt.Errorf("resource namespace does not match conditionBlock"))
		}
	}
	if conditionBlock.Selector != nil {
		hasPassed, err := checkSelector(conditionBlock.Selector, resource.GetLabels())
		if err != nil {
			errs = append(errs, fmt.Errorf("could not parse selector block of the policy in conditionBlock: %v", err))
		} else {
			if !hasPassed {
				errs = append(errs, fmt.Errorf("resource does not match selector of given conditionBlock"))
			}
		}
	}
	if len(userInfo.Roles) > 0 {
		if !doesSliceContainsAnyOfTheseValues(userInfo.Roles, admissionInfo.Roles...) {
			errs = append(errs, fmt.Errorf("user info does not match roles for the given conditionBlock"))
		}
	}
	if len(userInfo.ClusterRoles) > 0 {
		if !doesSliceContainsAnyOfTheseValues(userInfo.ClusterRoles, admissionInfo.ClusterRoles...) {
			errs = append(errs, fmt.Errorf("user info does not match clustersRoles for the given conditionBlock"))
		}
	}
	if len(userInfo.Subjects) > 0 {
		if !matchSubjects(userInfo.Subjects, admissionInfo.AdmissionUserInfo) {
			errs = append(errs, fmt.Errorf("user info does not match subject for the given conditionBlock"))
		}
	}

	return errs
}

// matchSubjects return true if one of ruleSubjects exist in userInfo
func matchSubjects(ruleSubjects []rbacv1.Subject, userInfo authenticationv1.UserInfo) bool {
	const SaPrefix = "system:serviceaccount:"

	userGroups := append(userInfo.Groups, userInfo.Username)
	for _, subject := range ruleSubjects {
		switch subject.Kind {
		case "ServiceAccount":
			if len(userInfo.Username) <= len(SaPrefix) {
				continue
			}
			subjectServiceAccount := subject.Namespace + ":" + subject.Name
			if userInfo.Username[len(SaPrefix):] == subjectServiceAccount {
				return true
			}
		case "User", "Group":
			if utils.ContainsString(userGroups, subject.Name) {
				return true
			}
		}
	}

	return false
}

func doesSliceContainsAnyOfTheseValues(slice []string, values ...string) bool {

	var sliceElementsMap = make(map[string]bool, len(slice))
	for _, sliceElement := range slice {
		sliceElementsMap[sliceElement] = true
	}

	for _, value := range values {
		if sliceElementsMap[value] {
			return true
		}
	}

	return false
}

//MatchesResourceDescription checks if the resource matches resource description of the rule or not
func MatchesResourceDescription(resourceRef unstructured.Unstructured, ruleRef kyverno.Rule, admissionInfoRef kyverno.RequestInfo) error {
	rule := *ruleRef.DeepCopy()
	resource := *resourceRef.DeepCopy()
	admissionInfo := *admissionInfoRef.DeepCopy()

	var reasonsForFailure []error

	if reflect.DeepEqual(admissionInfo, kyverno.RequestInfo{}) {
		rule.MatchResources.UserInfo = kyverno.UserInfo{}
	}

	// checking if resource matches the rule
	if !reflect.DeepEqual(rule.MatchResources.ResourceDescription, kyverno.ResourceDescription{}) {
		matchErrs := doesResourceMatchConditionBlock(rule.MatchResources.ResourceDescription, rule.MatchResources.UserInfo, admissionInfo, resource)
		reasonsForFailure = append(reasonsForFailure, matchErrs...)
	} else {
		reasonsForFailure = append(reasonsForFailure, fmt.Errorf("match block in rule cannot be empty"))
	}

	// checking if resource has been excluded
	if !reflect.DeepEqual(rule.ExcludeResources.ResourceDescription, kyverno.ResourceDescription{}) {
		excludeErrs := doesResourceMatchConditionBlock(rule.ExcludeResources.ResourceDescription, rule.ExcludeResources.UserInfo, admissionInfo, resource)
		if excludeErrs == nil {
			reasonsForFailure = append(reasonsForFailure, fmt.Errorf("resource has been excluded since it matches the exclude block"))
		}
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
func copyConditions(original []kyverno.Condition) []kyverno.Condition {
	var copy []kyverno.Condition
	for _, condition := range original {
		copy = append(copy, *condition.DeepCopy())
	}
	return copy
}
