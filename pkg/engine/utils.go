package engine

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	"github.com/kyverno/kyverno/pkg/engine/common"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/engine/wildcards"
	"github.com/kyverno/kyverno/pkg/logging"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	matched "github.com/kyverno/kyverno/pkg/utils/match"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

// EngineStats stores in the statistics for a single application of resource
type EngineStats struct {
	// average time required to process the policy rules on a resource
	ExecutionTime time.Duration
	// Count of rules that were applied successfully
	RulesAppliedCount int
}

func checkName(name, resourceName string) bool {
	return wildcard.Match(name, resourceName)
}

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

func checkAnnotations(annotations map[string]string, resourceAnnotations map[string]string) bool {
	if len(annotations) == 0 {
		return true
	}

	for k, v := range annotations {
		match := false
		for k1, v1 := range resourceAnnotations {
			if wildcard.Match(k, k1) && wildcard.Match(v, v1) {
				match = true
				break
			}
		}

		if !match {
			return false
		}
	}

	return true
}

func checkSelector(labelSelector *metav1.LabelSelector, resourceLabels map[string]string) (bool, error) {
	wildcards.ReplaceInSelector(labelSelector, resourceLabels)
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		logging.Error(err, "failed to build label selector")
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
func doesResourceMatchConditionBlock(subresourceGVKToAPIResource map[string]*metav1.APIResource, conditionBlock kyvernov1.ResourceDescription, userInfo kyvernov1.UserInfo, admissionInfo kyvernov1beta1.RequestInfo, resource unstructured.Unstructured, dynamicConfig []string, namespaceLabels map[string]string, subresourceInAdmnReview string) []error {
	var errs []error

	if len(conditionBlock.Kinds) > 0 {
		// Matching on ephemeralcontainers even when they are not explicitly specified for backward compatibility.
		if !matched.CheckKind(subresourceGVKToAPIResource, conditionBlock.Kinds, resource.GroupVersionKind(), subresourceInAdmnReview, true) {
			errs = append(errs, fmt.Errorf("kind does not match %v", conditionBlock.Kinds))
		}
	}

	resourceName := resource.GetName()
	if resourceName == "" {
		resourceName = resource.GetGenerateName()
	}

	if conditionBlock.Name != "" {
		if !checkName(conditionBlock.Name, resourceName) {
			errs = append(errs, fmt.Errorf("name does not match"))
		}
	}

	if len(conditionBlock.Names) > 0 {
		noneMatch := true
		for i := range conditionBlock.Names {
			if checkName(conditionBlock.Names[i], resourceName) {
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
		if !checkAnnotations(conditionBlock.Annotations, resource.GetAnnotations()) {
			errs = append(errs, fmt.Errorf("annotations does not match"))
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

	if conditionBlock.NamespaceSelector != nil && resource.GetKind() != "Namespace" &&
		(resource.GetKind() != "" || slices.Contains(conditionBlock.Kinds, "*") && wildcard.Match("*", resource.GetKind())) {
		hasPassed, err := checkSelector(conditionBlock.NamespaceSelector, namespaceLabels)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to parse namespace selector: %v", err))
		} else {
			if !hasPassed {
				errs = append(errs, fmt.Errorf("namespace selector does not match"))
			}
		}
	}

	keys := append(admissionInfo.AdmissionUserInfo.Groups, admissionInfo.AdmissionUserInfo.Username)
	var userInfoErrors []error
	if len(userInfo.Roles) > 0 && !datautils.SliceContains(keys, dynamicConfig...) {
		if !datautils.SliceContains(userInfo.Roles, admissionInfo.Roles...) {
			userInfoErrors = append(userInfoErrors, fmt.Errorf("user info does not match roles for the given conditionBlock"))
		}
	}

	if len(userInfo.ClusterRoles) > 0 && !datautils.SliceContains(keys, dynamicConfig...) {
		if !datautils.SliceContains(userInfo.ClusterRoles, admissionInfo.ClusterRoles...) {
			userInfoErrors = append(userInfoErrors, fmt.Errorf("user info does not match clustersRoles for the given conditionBlock"))
		}
	}

	if len(userInfo.Subjects) > 0 {
		if !matchSubjects(userInfo.Subjects, admissionInfo.AdmissionUserInfo, dynamicConfig) {
			userInfoErrors = append(userInfoErrors, fmt.Errorf("user info does not match subject for the given conditionBlock"))
		}
	}
	return append(errs, userInfoErrors...)
}

// matchSubjects return true if one of ruleSubjects exist in userInfo
func matchSubjects(ruleSubjects []rbacv1.Subject, userInfo authenticationv1.UserInfo, dynamicConfig []string) bool {
	const SaPrefix = "system:serviceaccount:"

	if store.GetMock() {
		mockSubject := store.GetSubjects().Subject
		for _, subject := range ruleSubjects {
			switch subject.Kind {
			case "ServiceAccount":
				if subject.Name == mockSubject.Name && subject.Namespace == mockSubject.Namespace {
					return true
				}
			case "User", "Group":
				if mockSubject.Name == subject.Name {
					return true
				}
			}
		}

		return false
	} else {
		userGroups := append(userInfo.Groups, userInfo.Username)
		// TODO: see issue https://github.com/kyverno/kyverno/issues/861
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
				if slices.Contains(userGroups, subject.Name) {
					return true
				}
			}
		}

		return false
	}
}

// MatchesResourceDescription checks if the resource matches resource description of the rule or not
func MatchesResourceDescription(subresourceGVKToAPIResource map[string]*metav1.APIResource, resourceRef unstructured.Unstructured, ruleRef kyvernov1.Rule, admissionInfoRef kyvernov1beta1.RequestInfo, dynamicConfig []string, namespaceLabels map[string]string, policyNamespace, subresourceInAdmnReview string) error {
	rule := ruleRef.DeepCopy()
	resource := *resourceRef.DeepCopy()
	admissionInfo := *admissionInfoRef.DeepCopy()
	empty := []string{}

	var reasonsForFailure []error
	if policyNamespace != "" && policyNamespace != resourceRef.GetNamespace() {
		return errors.New(" The policy and resource namespace are different. Therefore, policy skip this resource.")
	}

	if len(rule.MatchResources.Any) > 0 {
		// include object if ANY of the criteria match
		// so if one matches then break from loop
		oneMatched := false
		for _, rmr := range rule.MatchResources.Any {
			// if there are no errors it means it was a match
			if len(matchesResourceDescriptionMatchHelper(subresourceGVKToAPIResource, rmr, admissionInfo, resource, empty, namespaceLabels, subresourceInAdmnReview)) == 0 {
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
			reasonsForFailure = append(reasonsForFailure, matchesResourceDescriptionMatchHelper(subresourceGVKToAPIResource, rmr, admissionInfo, resource, empty, namespaceLabels, subresourceInAdmnReview)...)
		}
	} else {
		rmr := kyvernov1.ResourceFilter{UserInfo: rule.MatchResources.UserInfo, ResourceDescription: rule.MatchResources.ResourceDescription}
		reasonsForFailure = append(reasonsForFailure, matchesResourceDescriptionMatchHelper(subresourceGVKToAPIResource, rmr, admissionInfo, resource, empty, namespaceLabels, subresourceInAdmnReview)...)
	}

	if len(rule.ExcludeResources.Any) > 0 {
		// exclude the object if ANY of the criteria match
		for _, rer := range rule.ExcludeResources.Any {
			reasonsForFailure = append(reasonsForFailure, matchesResourceDescriptionExcludeHelper(subresourceGVKToAPIResource, rer, admissionInfo, resource, dynamicConfig, namespaceLabels, subresourceInAdmnReview)...)
		}
	} else if len(rule.ExcludeResources.All) > 0 {
		// exclude the object if ALL the criteria match
		excludedByAll := true
		for _, rer := range rule.ExcludeResources.All {
			// we got no errors inplying a resource did NOT exclude it
			// "matchesResourceDescriptionExcludeHelper" returns errors if resource is excluded by a filter
			if len(matchesResourceDescriptionExcludeHelper(subresourceGVKToAPIResource, rer, admissionInfo, resource, dynamicConfig, namespaceLabels, subresourceInAdmnReview)) == 0 {
				excludedByAll = false
				break
			}
		}
		if excludedByAll {
			reasonsForFailure = append(reasonsForFailure, fmt.Errorf("resource excluded since the combination of all criteria exclude it"))
		}
	} else {
		rer := kyvernov1.ResourceFilter{UserInfo: rule.ExcludeResources.UserInfo, ResourceDescription: rule.ExcludeResources.ResourceDescription}
		reasonsForFailure = append(reasonsForFailure, matchesResourceDescriptionExcludeHelper(subresourceGVKToAPIResource, rer, admissionInfo, resource, dynamicConfig, namespaceLabels, subresourceInAdmnReview)...)
	}

	// creating final error
	errorMessage := fmt.Sprintf("rule %s not matched:", ruleRef.Name)
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

func matchesResourceDescriptionMatchHelper(subresourceGVKToAPIResource map[string]*metav1.APIResource, rmr kyvernov1.ResourceFilter, admissionInfo kyvernov1beta1.RequestInfo, resource unstructured.Unstructured, dynamicConfig []string, namespaceLabels map[string]string, subresourceInAdmnReview string) []error {
	var errs []error
	if reflect.DeepEqual(admissionInfo, kyvernov1.RequestInfo{}) {
		rmr.UserInfo = kyvernov1.UserInfo{}
	}

	// checking if resource matches the rule
	if !reflect.DeepEqual(rmr.ResourceDescription, kyvernov1.ResourceDescription{}) ||
		!reflect.DeepEqual(rmr.UserInfo, kyvernov1.UserInfo{}) {
		matchErrs := doesResourceMatchConditionBlock(subresourceGVKToAPIResource, rmr.ResourceDescription, rmr.UserInfo, admissionInfo, resource, dynamicConfig, namespaceLabels, subresourceInAdmnReview)
		errs = append(errs, matchErrs...)
	} else {
		errs = append(errs, fmt.Errorf("match cannot be empty"))
	}
	return errs
}

func matchesResourceDescriptionExcludeHelper(subresourceGVKToAPIResource map[string]*metav1.APIResource, rer kyvernov1.ResourceFilter, admissionInfo kyvernov1beta1.RequestInfo, resource unstructured.Unstructured, dynamicConfig []string, namespaceLabels map[string]string, subresourceInAdmnReview string) []error {
	var errs []error
	// checking if resource matches the rule
	if !reflect.DeepEqual(rer.ResourceDescription, kyvernov1.ResourceDescription{}) ||
		!reflect.DeepEqual(rer.UserInfo, kyvernov1.UserInfo{}) {
		excludeErrs := doesResourceMatchConditionBlock(subresourceGVKToAPIResource, rer.ResourceDescription, rer.UserInfo, admissionInfo, resource, dynamicConfig, namespaceLabels, subresourceInAdmnReview)
		// it was a match so we want to exclude it
		if len(excludeErrs) == 0 {
			errs = append(errs, fmt.Errorf("resource excluded since one of the criteria excluded it"))
			errs = append(errs, excludeErrs...)
		}
	}
	// len(errs) != 0 if the filter excluded the resource
	return errs
}

// excludeResource checks if the resource has ownerRef set
func excludeResource(podControllers string, resource unstructured.Unstructured) bool {
	kind := resource.GetKind()
	hasOwner := false
	if kind == "Pod" || kind == "Job" {
		for _, owner := range resource.GetOwnerReferences() {
			hasOwner = true
			if owner.Kind != "ReplicaSet" && !strings.Contains(podControllers, owner.Kind) {
				return false
			}
		}
		return hasOwner
	}

	return false
}

// ManagedPodResource returns true:
// - if the policy has auto-gen annotation && resource == Pod
// - if the auto-gen contains cronJob && resource == Job
func ManagedPodResource(policy kyvernov1.PolicyInterface, resource unstructured.Unstructured) bool {
	podControllers, ok := policy.GetAnnotations()[kyvernov1.PodControllersAnnotation]
	if !ok || strings.ToLower(podControllers) == "none" {
		return false
	}

	if excludeResource(podControllers, resource) {
		return true
	}

	if strings.Contains(podControllers, "CronJob") && excludeResource(podControllers, resource) {
		return true
	}

	return false
}

func checkPreconditions(logger logr.Logger, ctx *PolicyContext, anyAllConditions apiextensions.JSON) (bool, error) {
	preconditions, err := variables.SubstituteAllInPreconditions(logger, ctx.jsonContext, anyAllConditions)
	if err != nil {
		return false, errors.Wrapf(err, "failed to substitute variables in preconditions")
	}

	typeConditions, err := common.TransformConditions(preconditions)
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse preconditions")
	}

	pass := variables.EvaluateConditions(logger, ctx.jsonContext, typeConditions)
	return pass, nil
}

func evaluateList(jmesPath string, ctx context.EvalInterface) ([]interface{}, error) {
	i, err := ctx.Query(jmesPath)
	if err != nil {
		return nil, err
	}

	l, ok := i.([]interface{})
	if !ok {
		return []interface{}{i}, nil
	}

	return l, nil
}

func ruleError(rule *kyvernov1.Rule, ruleType response.RuleType, msg string, err error) *response.RuleResponse {
	msg = fmt.Sprintf("%s: %s", msg, err.Error())
	return ruleResponse(*rule, ruleType, msg, response.RuleStatusError)
}

func ruleResponse(rule kyvernov1.Rule, ruleType response.RuleType, msg string, status response.RuleStatus) *response.RuleResponse {
	resp := &response.RuleResponse{
		Name:    rule.Name,
		Type:    ruleType,
		Message: msg,
		Status:  status,
	}
	return resp
}

func incrementAppliedCount(resp *response.EngineResponse) {
	resp.PolicyResponse.RulesAppliedCount++
}

func incrementErrorCount(resp *response.EngineResponse) {
	resp.PolicyResponse.RulesErrorCount++
}

// invertedElement inverted the order of element for patchStrategicMerge  policies as kustomize patch revering the order of patch resources.
func invertedElement(elements []interface{}) {
	for i, j := 0, len(elements)-1; i < j; i, j = i+1, j-1 {
		elements[i], elements[j] = elements[j], elements[i]
	}
}
