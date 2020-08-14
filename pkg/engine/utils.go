package engine

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/nirmata/kyverno/pkg/utils"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

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
		if wildcard.Match(namespace, resourceNameSpace) {
			return true
		}
	}
	return false
}

func checkSelector(labelSelector *metav1.LabelSelector, resourceLabels map[string]string) (bool, error) {
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		log.Log.Error(err, "failed to build label selector")
		return false, err
	}

	if selector.Matches(labels.Set(resourceLabels)) {
		return true, nil
	}

	return false, nil
}

// doesResourceMatchConditionBlock filters the resource with defined conditions
// for a match / exclude block, it has the following attributes:
// ResourceDescription:
// 		Kinds      []string
// 		Name       string
// 		Namespaces []string
// 		Selector
// UserInfo:
// 		Roles        []string
// 		ClusterRoles []string
// 		Subjects     []rbacv1.Subject
// To filter out the targeted resources with ResourceDescription, the check
// should be: AND across attibutes but an OR inside attributes that of type list
// To filter out the targeted resources with UserInfo, the check
// should be: OR (accross & inside) attributes
func doesResourceMatchConditionBlock(conditionBlock kyverno.ResourceDescription, userInfo kyverno.UserInfo, admissionInfo kyverno.RequestInfo, resource unstructured.Unstructured, dynamicConfig []string) []error {
	var errs []error
	if len(conditionBlock.Kinds) > 0 {
		if !checkKind(conditionBlock.Kinds, resource.GetKind()) {
			errs = append(errs, fmt.Errorf("kind does not match"))
		}
	}
	if conditionBlock.Name != "" {
		if !checkName(conditionBlock.Name, resource.GetName()) {
			errs = append(errs, fmt.Errorf("name does not match"))
		}
	}
	if len(conditionBlock.Namespaces) > 0 {
		if !checkNameSpace(conditionBlock.Namespaces, resource.GetNamespace()) {
			errs = append(errs, fmt.Errorf("namespace does not match"))
		}
	}
	if conditionBlock.Selector != nil {
		hasPassed, err := checkSelector(conditionBlock.Selector, resource.GetLabels())
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to parse selector: %v", err))
		} else {
			if !hasPassed {
				errs = append(errs, fmt.Errorf("selector does not match"))
			}
		}
	}

	keys := append(admissionInfo.AdmissionUserInfo.Groups, admissionInfo.AdmissionUserInfo.Username)
	var userInfoErrors []error
	var checkedItem int
	if len(userInfo.Roles) > 0 && !utils.SliceContains(keys, dynamicConfig...) {
		checkedItem++

		if !utils.SliceContains(userInfo.Roles, admissionInfo.Roles...) {
			userInfoErrors = append(userInfoErrors, fmt.Errorf("user info does not match roles for the given conditionBlock"))
		} else {
			return errs
		}
	}

	if len(userInfo.ClusterRoles) > 0 && !utils.SliceContains(keys, dynamicConfig...) {
		checkedItem++

		if !utils.SliceContains(userInfo.ClusterRoles, admissionInfo.ClusterRoles...) {
			userInfoErrors = append(userInfoErrors, fmt.Errorf("user info does not match clustersRoles for the given conditionBlock"))
		} else {
			return errs
		}
	}

	if len(userInfo.Subjects) > 0 {
		checkedItem++

		if !matchSubjects(userInfo.Subjects, admissionInfo.AdmissionUserInfo, dynamicConfig) {
			userInfoErrors = append(userInfoErrors, fmt.Errorf("user info does not match subject for the given conditionBlock"))
		} else {
			return errs
		}
	}

	if checkedItem != len(userInfoErrors) {
		return errs
	}

	return append(errs, userInfoErrors...)
}

// matchSubjects return true if one of ruleSubjects exist in userInfo
func matchSubjects(ruleSubjects []rbacv1.Subject, userInfo authenticationv1.UserInfo, dynamicConfig []string) bool {
	const SaPrefix = "system:serviceaccount:"

	userGroups := append(userInfo.Groups, userInfo.Username)

	// TODO: see issue https://github.com/nirmata/kyverno/issues/861
	for _, e := range dynamicConfig {
		ruleSubjects = append(ruleSubjects,
			rbacv1.Subject{Kind: "Group", Name: e},
		)
	}

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

//MatchesResourceDescription checks if the resource matches resource description of the rule or not
func MatchesResourceDescription(resourceRef unstructured.Unstructured, ruleRef kyverno.Rule, admissionInfoRef kyverno.RequestInfo, dynamicConfig []string) error {

	rule := *ruleRef.DeepCopy()
	resource := *resourceRef.DeepCopy()
	admissionInfo := *admissionInfoRef.DeepCopy()

	var reasonsForFailure []error

	if reflect.DeepEqual(admissionInfo, kyverno.RequestInfo{}) {
		rule.MatchResources.UserInfo = kyverno.UserInfo{}
	}

	// checking if resource matches the rule
	if !reflect.DeepEqual(rule.MatchResources.ResourceDescription, kyverno.ResourceDescription{}) ||
		!reflect.DeepEqual(rule.MatchResources.UserInfo, kyverno.UserInfo{}) {
		matchErrs := doesResourceMatchConditionBlock(rule.MatchResources.ResourceDescription, rule.MatchResources.UserInfo, admissionInfo, resource, dynamicConfig)
		reasonsForFailure = append(reasonsForFailure, matchErrs...)
	} else {
		reasonsForFailure = append(reasonsForFailure, fmt.Errorf("match cannot be empty"))
	}

	// checking if resource has been excluded
	if !reflect.DeepEqual(rule.ExcludeResources.ResourceDescription, kyverno.ResourceDescription{}) ||
		!reflect.DeepEqual(rule.ExcludeResources.UserInfo, kyverno.UserInfo{}) {
		excludeErrs := doesResourceMatchConditionBlock(rule.ExcludeResources.ResourceDescription, rule.ExcludeResources.UserInfo, admissionInfo, resource, dynamicConfig)
		if excludeErrs == nil {
			reasonsForFailure = append(reasonsForFailure, fmt.Errorf("resource excluded"))
		}
	}

	// creating final error
	var errorMessage = "rule not matched:"
	for i, reasonForFailure := range reasonsForFailure {
		if reasonForFailure != nil {
			errorMessage += "\n " + fmt.Sprint(i+1) + ". " + reasonForFailure.Error()
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

// excludePod checks if a Pod has ownerRef set
func excludePod(resource unstructured.Unstructured) bool {
	if resource.GetKind() == "Pod" {
		if len(resource.GetOwnerReferences()) > 0 {
			return true
		}
	}

	return false
}
