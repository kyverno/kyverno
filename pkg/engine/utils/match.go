package utils

import (
	"fmt"
	"slices"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	matchutils "github.com/kyverno/kyverno/pkg/utils/match"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func checkNameSpace(namespaces []string, resource unstructured.Unstructured) bool {
	resourceNameSpace := resource.GetNamespace()
	if resource.GetKind() == "Namespace" {
		resourceNameSpace = resource.GetName()
	}

	for _, namespace := range namespaces {
		if wildcard.Match(namespace, resourceNameSpace) {
			return true
		}
	}

	return false
}

// doesResourceMatchConditionBlock filters the resource with defined conditions
// for a match / exclude block, it has the following attributes:
// ResourceDescription:
//
//	Kinds      []string
//	Name       string
//	Namespaces []string
//	Selector
//
// UserInfo:
//
//	Roles        []string
//	ClusterRoles []string
//	Subjects     []rbacv1.Subject
//
// To filter out the targeted resources with ResourceDescription, the check
// should be: AND across attributes but an OR inside attributes that of type list
// To filter out the targeted resources with UserInfo, the check
// should be: OR (across & inside) attributes
func doesResourceMatchConditionBlock(
	conditionBlock kyvernov1.ResourceDescription,
	userInfo kyvernov1.UserInfo,
	admissionInfo kyvernov1beta1.RequestInfo,
	resource unstructured.Unstructured,
	namespaceLabels map[string]string,
	gvk schema.GroupVersionKind,
	subresource string,
	operation kyvernov1.AdmissionOperation,
) []error {
	if len(conditionBlock.Operations) > 0 {
		if !slices.Contains(conditionBlock.Operations, operation) {
			// if operation does not match, return immediately
			err := fmt.Errorf("operation does not match")
			return []error{err}
		}
	}

	var errs []error
	if len(conditionBlock.Kinds) > 0 {
		// Matching on ephemeralcontainers even when they are not explicitly specified for backward compatibility.
		if !matchutils.CheckKind(conditionBlock.Kinds, gvk, subresource, true) {
			errs = append(errs, fmt.Errorf("kind does not match %v", conditionBlock.Kinds))
		}
	}

	resourceName := resource.GetName()
	if resourceName == "" {
		resourceName = resource.GetGenerateName()
	}

	if conditionBlock.Name != "" {
		if !matchutils.CheckName(conditionBlock.Name, resourceName) {
			errs = append(errs, fmt.Errorf("name does not match"))
		}
	}

	if len(conditionBlock.Names) > 0 {
		noneMatch := true
		for i := range conditionBlock.Names {
			if matchutils.CheckName(conditionBlock.Names[i], resourceName) {
				noneMatch = false
				break
			}
		}
		if noneMatch {
			errs = append(errs, fmt.Errorf("none of the names match"))
		}
	}

	if len(conditionBlock.Namespaces) > 0 {
		if !checkNameSpace(conditionBlock.Namespaces, resource) {
			errs = append(errs, fmt.Errorf("namespace does not match"))
		}
	}

	if len(conditionBlock.Annotations) > 0 {
		if !matchutils.CheckAnnotations(conditionBlock.Annotations, resource.GetAnnotations()) {
			errs = append(errs, fmt.Errorf("annotations does not match"))
		}
	}

	if conditionBlock.Selector != nil {
		hasPassed, err := matchutils.CheckSelector(conditionBlock.Selector, resource.GetLabels())
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to parse selector: %v", err))
		} else {
			if !hasPassed {
				errs = append(errs, fmt.Errorf("selector does not match"))
			}
		}
	}

	if conditionBlock.NamespaceSelector != nil {
		if resource.GetKind() == "Namespace" {
			errs = append(errs, fmt.Errorf("namespace selector is not applicable for namespace resource"))
		} else if resource.GetKind() != "" || slices.Contains(conditionBlock.Kinds, "*") && wildcard.Match("*", resource.GetKind()) {
			hasPassed, err := matchutils.CheckSelector(conditionBlock.NamespaceSelector, namespaceLabels)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to parse namespace selector: %v", err))
			} else {
				if !hasPassed {
					errs = append(errs, fmt.Errorf("namespace selector does not match labels"))
				}
			}
		}
	}

	var userInfoErrors []error
	if len(userInfo.Roles) > 0 {
		if !datautils.SliceContains(userInfo.Roles, admissionInfo.Roles...) {
			userInfoErrors = append(userInfoErrors, fmt.Errorf("user info does not match roles for the given conditionBlock"))
		}
	}

	if len(userInfo.ClusterRoles) > 0 {
		if !datautils.SliceContains(userInfo.ClusterRoles, admissionInfo.ClusterRoles...) {
			userInfoErrors = append(userInfoErrors, fmt.Errorf("user info does not match clustersRoles for the given conditionBlock"))
		}
	}

	if len(userInfo.Subjects) > 0 {
		if !matchSubjects(userInfo.Subjects, admissionInfo.AdmissionUserInfo) {
			userInfoErrors = append(userInfoErrors, fmt.Errorf("user info does not match subject for the given conditionBlock"))
		}
	}

	return append(errs, userInfoErrors...)
}

// matchSubjects return true if one of ruleSubjects exist in userInfo
func matchSubjects(ruleSubjects []rbacv1.Subject, userInfo authenticationv1.UserInfo) bool {
	return matchutils.CheckSubjects(ruleSubjects, userInfo)
}

// matchesResourceDescription checks if the resource matches resource description of the rule or not
func MatchesResourceDescription(
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	admissionInfo kyvernov1beta1.RequestInfo,
	namespaceLabels map[string]string,
	policyNamespace string,
	gvk schema.GroupVersionKind,
	subresource string,
	operation kyvernov1.AdmissionOperation,
) error {
	if resource.Object == nil {
		return fmt.Errorf("resource is empty")
	}

	var reasonsForFailure []error
	if policyNamespace != "" && policyNamespace != resource.GetNamespace() {
		return fmt.Errorf("policy and resource namespaces mismatch")
	}

	if len(rule.MatchResources.Any) > 0 {
		// include object if ANY of the criteria match
		// so if one matches then break from loop
		oneMatched := false
		for _, rmr := range rule.MatchResources.Any {
			// if there are no errors it means it was a match
			if len(matchesResourceDescriptionMatchHelper(rmr, admissionInfo, resource, namespaceLabels, gvk, subresource, operation)) == 0 {
				oneMatched = true
				break
			}
		}
		if !oneMatched {
			reasonsForFailure = append(reasonsForFailure, fmt.Errorf("no resource matched"))
		}
	} else if len(rule.MatchResources.All) > 0 {
		// include object if ALL of the criteria match
		for _, rmr := range rule.MatchResources.All {
			reasonsForFailure = append(reasonsForFailure, matchesResourceDescriptionMatchHelper(rmr, admissionInfo, resource, namespaceLabels, gvk, subresource, operation)...)
		}
	} else {
		rmr := kyvernov1.ResourceFilter{UserInfo: rule.MatchResources.UserInfo, ResourceDescription: rule.MatchResources.ResourceDescription}
		reasonsForFailure = append(reasonsForFailure, matchesResourceDescriptionMatchHelper(rmr, admissionInfo, resource, namespaceLabels, gvk, subresource, operation)...)
	}

	// check exlude conditions only if match succeeds
	if len(reasonsForFailure) == 0 {
		if len(rule.ExcludeResources.Any) > 0 {
			// exclude the object if ANY of the criteria match
			for _, rer := range rule.ExcludeResources.Any {
				reasonsForFailure = append(reasonsForFailure, matchesResourceDescriptionExcludeHelper(rer, admissionInfo, resource, namespaceLabels, gvk, subresource, operation)...)
			}
		} else if len(rule.ExcludeResources.All) > 0 {
			// exclude the object if ALL the criteria match
			excludedByAll := true
			for _, rer := range rule.ExcludeResources.All {
				// we got no errors inplying a resource did NOT exclude it
				// "matchesResourceDescriptionExcludeHelper" returns errors if resource is excluded by a filter
				if len(matchesResourceDescriptionExcludeHelper(rer, admissionInfo, resource, namespaceLabels, gvk, subresource, operation)) == 0 {
					excludedByAll = false
					break
				}
			}
			if excludedByAll {
				reasonsForFailure = append(reasonsForFailure, fmt.Errorf("resource excluded since the combination of all criteria exclude it"))
			}
		} else {
			rer := kyvernov1.ResourceFilter{UserInfo: rule.ExcludeResources.UserInfo, ResourceDescription: rule.ExcludeResources.ResourceDescription}
			reasonsForFailure = append(reasonsForFailure, matchesResourceDescriptionExcludeHelper(rer, admissionInfo, resource, namespaceLabels, gvk, subresource, operation)...)
		}
	}

	// creating final error
	errorMessage := fmt.Sprintf("rule %s not matched:", rule.Name)
	for i, reasonForFailure := range reasonsForFailure {
		if reasonForFailure != nil {
			errorMessage += "\n " + fmt.Sprint(i+1) + ". " + reasonForFailure.Error()
		}
	}

	if len(reasonsForFailure) > 0 {
		return fmt.Errorf(errorMessage)
	}

	return nil
}

func matchesResourceDescriptionMatchHelper(
	rmr kyvernov1.ResourceFilter,
	admissionInfo kyvernov1beta1.RequestInfo,
	resource unstructured.Unstructured,
	namespaceLabels map[string]string,
	gvk schema.GroupVersionKind,
	subresource string,
	operation kyvernov1.AdmissionOperation,
) []error {
	var errs []error
	if datautils.DeepEqual(admissionInfo, kyvernov1beta1.RequestInfo{}) {
		rmr.UserInfo = kyvernov1.UserInfo{}
	}

	// checking if resource matches the rule
	if !datautils.DeepEqual(rmr.ResourceDescription, kyvernov1.ResourceDescription{}) ||
		!datautils.DeepEqual(rmr.UserInfo, kyvernov1.UserInfo{}) {
		matchErrs := doesResourceMatchConditionBlock(rmr.ResourceDescription, rmr.UserInfo, admissionInfo, resource, namespaceLabels, gvk, subresource, operation)
		errs = append(errs, matchErrs...)
	} else {
		errs = append(errs, fmt.Errorf("match cannot be empty"))
	}
	return errs
}

func matchesResourceDescriptionExcludeHelper(
	rer kyvernov1.ResourceFilter,
	admissionInfo kyvernov1beta1.RequestInfo,
	resource unstructured.Unstructured,
	namespaceLabels map[string]string,
	gvk schema.GroupVersionKind,
	subresource string,
	operation kyvernov1.AdmissionOperation,
) []error {
	var errs []error
	// checking if resource matches the rule
	if !datautils.DeepEqual(rer.ResourceDescription, kyvernov1.ResourceDescription{}) ||
		!datautils.DeepEqual(rer.UserInfo, kyvernov1.UserInfo{}) {
		excludeErrs := doesResourceMatchConditionBlock(rer.ResourceDescription, rer.UserInfo, admissionInfo, resource, namespaceLabels, gvk, subresource, operation)
		// it was a match so we want to exclude it
		if len(excludeErrs) == 0 {
			errs = append(errs, fmt.Errorf("resource excluded since one of the criteria excluded it"))
			errs = append(errs, excludeErrs...)
		}
	}
	// len(errs) != 0 if the filter excluded the resource
	return errs
}
