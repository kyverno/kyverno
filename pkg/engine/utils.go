package engine

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	engineUtils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/pkg/errors"

	"github.com/kyverno/kyverno/pkg/engine/wildcards"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/minio/pkg/wildcard"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//EngineStats stores in the statistics for a single application of resource
type EngineStats struct {
	// average time required to process the policy rules on a resource
	ExecutionTime time.Duration
	// Count of rules that were applied successfully
	RulesAppliedCount int
}

func checkKind(kinds []string, resource unstructured.Unstructured) bool {
	for _, kind := range kinds {
		SplitGVK := strings.Split(kind, "/")
		if len(SplitGVK) == 1 {
			if resource.GetKind() == strings.Title(kind) || kind == "*" {
				return true
			}
		} else if len(SplitGVK) == 2 {
			if resource.GroupVersionKind().Kind == strings.Title(SplitGVK[1]) && resource.GroupVersionKind().Version == SplitGVK[0] {
				return true
			}
		} else {
			if resource.GroupVersionKind().Group == SplitGVK[0] && resource.GroupVersionKind().Kind == strings.Title(SplitGVK[2]) && (resource.GroupVersionKind().Version == SplitGVK[1] || resource.GroupVersionKind().Version == "*") {
				return true
			}
		}
	}

	return false
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

		if match == false {
			return false
		}
	}

	return true
}

func checkSelector(labelSelector *metav1.LabelSelector, resourceLabels map[string]string) (bool, error) {
	wildcards.ReplaceInSelector(labelSelector, resourceLabels)
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
// should be: AND across attributes but an OR inside attributes that of type list
// To filter out the targeted resources with UserInfo, the check
// should be: OR (across & inside) attributes
func doesResourceMatchConditionBlock(conditionBlock kyverno.ResourceDescription, userInfo kyverno.UserInfo, admissionInfo kyverno.RequestInfo, resource unstructured.Unstructured, dynamicConfig []string, namespaceLabels map[string]string) []error {
	var errs []error

	if len(conditionBlock.Kinds) > 0 {
		if !checkKind(conditionBlock.Kinds, resource) {
			errs = append(errs, fmt.Errorf("kind does not match %v", conditionBlock.Kinds))
		}
	}

	if conditionBlock.Name != "" {
		if !checkName(conditionBlock.Name, resource.GetName()) {
			errs = append(errs, fmt.Errorf("name does not match"))
		}
	}

	if len(conditionBlock.Names) > 0 {
		noneMatch := true
		for i := range conditionBlock.Names {
			if checkName(conditionBlock.Names[i], resource.GetName()) {
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

	if conditionBlock.NamespaceSelector != nil && resource.GetKind() != "Namespace" && resource.GetKind() != "" {
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
			if utils.ContainsString(userGroups, subject.Name) {
				return true
			}
		}
	}

	return false
}

//MatchesResourceDescription checks if the resource matches resource description of the rule or not
func MatchesResourceDescription(resourceRef unstructured.Unstructured, ruleRef kyverno.Rule, admissionInfoRef kyverno.RequestInfo, dynamicConfig []string, namespaceLabels map[string]string, policyNamespace string) error {

	rule := ruleRef.DeepCopy()
	resource := *resourceRef.DeepCopy()
	admissionInfo := *admissionInfoRef.DeepCopy()

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
			if len(matchesResourceDescriptionMatchHelper(rmr, admissionInfo, resource, dynamicConfig, namespaceLabels)) == 0 {
				oneMatched = true
				break
			}
		}
		if !oneMatched {
			reasonsForFailure = append(reasonsForFailure, fmt.Errorf("no resource matched"))
		}
	} else if len(rule.MatchResources.All) > 0 {
		// include object if ALL of the criterias match
		for _, rmr := range rule.MatchResources.All {
			reasonsForFailure = append(reasonsForFailure, matchesResourceDescriptionMatchHelper(rmr, admissionInfo, resource, dynamicConfig, namespaceLabels)...)
		}
	} else {
		rmr := kyverno.ResourceFilter{UserInfo: rule.MatchResources.UserInfo, ResourceDescription: rule.MatchResources.ResourceDescription}
		reasonsForFailure = append(reasonsForFailure, matchesResourceDescriptionMatchHelper(rmr, admissionInfo, resource, dynamicConfig, namespaceLabels)...)
	}

	if len(rule.ExcludeResources.Any) > 0 {
		// exclude the object if ANY of the criterias match
		for _, rer := range rule.ExcludeResources.Any {
			reasonsForFailure = append(reasonsForFailure, matchesResourceDescriptionExcludeHelper(rer, admissionInfo, resource, dynamicConfig, namespaceLabels)...)
		}
	} else if len(rule.ExcludeResources.All) > 0 {
		// exlcude the object if ALL the criterias match
		excludedByAll := true
		for _, rer := range rule.ExcludeResources.All {
			// we got no errors inplying a resource did NOT exclude it
			// "matchesResourceDescriptionExcludeHelper" returns errors if resource is excluded by a filter
			if len(matchesResourceDescriptionExcludeHelper(rer, admissionInfo, resource, dynamicConfig, namespaceLabels)) == 0 {
				excludedByAll = false
				break
			}
		}
		if excludedByAll {
			reasonsForFailure = append(reasonsForFailure, fmt.Errorf("resource excluded since the combination of all criterias exclude it"))
		}
	} else {
		rer := kyverno.ResourceFilter{UserInfo: rule.ExcludeResources.UserInfo, ResourceDescription: rule.ExcludeResources.ResourceDescription}
		reasonsForFailure = append(reasonsForFailure, matchesResourceDescriptionExcludeHelper(rer, admissionInfo, resource, dynamicConfig, namespaceLabels)...)
	}

	// creating final error
	var errorMessage = fmt.Sprintf("rule %s not matched:", ruleRef.Name)
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

func matchesResourceDescriptionMatchHelper(rmr kyverno.ResourceFilter, admissionInfo kyverno.RequestInfo, resource unstructured.Unstructured, dynamicConfig []string, namespaceLabels map[string]string) []error {
	var errs []error
	if reflect.DeepEqual(admissionInfo, kyverno.RequestInfo{}) {
		rmr.UserInfo = kyverno.UserInfo{}
	}

	// checking if resource matches the rule
	if !reflect.DeepEqual(rmr.ResourceDescription, kyverno.ResourceDescription{}) ||
		!reflect.DeepEqual(rmr.UserInfo, kyverno.UserInfo{}) {
		matchErrs := doesResourceMatchConditionBlock(rmr.ResourceDescription, rmr.UserInfo, admissionInfo, resource, dynamicConfig, namespaceLabels)
		errs = append(errs, matchErrs...)
	} else {
		errs = append(errs, fmt.Errorf("match cannot be empty"))
	}
	return errs
}

func matchesResourceDescriptionExcludeHelper(rer kyverno.ResourceFilter, admissionInfo kyverno.RequestInfo, resource unstructured.Unstructured, dynamicConfig []string, namespaceLabels map[string]string) []error {
	var errs []error
	// checking if resource matches the rule
	if !reflect.DeepEqual(rer.ResourceDescription, kyverno.ResourceDescription{}) ||
		!reflect.DeepEqual(rer.UserInfo, kyverno.UserInfo{}) {
		excludeErrs := doesResourceMatchConditionBlock(rer.ResourceDescription, rer.UserInfo, admissionInfo, resource, dynamicConfig, namespaceLabels)
		// it was a match so we want to exclude it
		if len(excludeErrs) == 0 {
			errs = append(errs, fmt.Errorf("resource excluded since one of the criterias excluded it"))
			errs = append(errs, excludeErrs...)
		}
	}
	// len(errs) != 0 if the filter excluded the resource
	return errs
}

func copyAnyAllConditions(original kyverno.AnyAllConditions) kyverno.AnyAllConditions {
	if reflect.DeepEqual(original, kyverno.AnyAllConditions{}) {
		return kyverno.AnyAllConditions{}
	}
	return *original.DeepCopy()
}

// backwards compatibility
func copyOldConditions(original []kyverno.Condition) []kyverno.Condition {
	if original == nil || len(original) == 0 {
		return []kyverno.Condition{}
	}

	var copies []kyverno.Condition
	for _, condition := range original {
		copies = append(copies, *condition.DeepCopy())
	}

	return copies
}

func transformConditions(original apiextensions.JSON) (interface{}, error) {
	// conditions are currently in the form of []interface{}
	kyvernoOriginalConditions, err := utils.ApiextensionsJsonToKyvernoConditions(original)
	if err != nil {
		return nil, err
	}
	switch typedValue := kyvernoOriginalConditions.(type) {
	case kyverno.AnyAllConditions:
		return copyAnyAllConditions(typedValue), nil
	case []kyverno.Condition: // backwards compatibility
		return copyOldConditions(typedValue), nil
	}

	return nil, fmt.Errorf("invalid preconditions")
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
func ManagedPodResource(policy kyverno.ClusterPolicy, resource unstructured.Unstructured) bool {
	podControllers, ok := policy.GetAnnotations()[PodControllersAnnotation]
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
	preconditions, err := variables.SubstituteAllInPreconditions(logger, ctx.JSONContext, anyAllConditions)
	if err != nil {
		return false, errors.Wrapf(err, "failed to substitute variables in preconditions")
	}

	typeConditions, err := transformConditions(preconditions)
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse preconditions")
	}

	pass := variables.EvaluateConditions(logger, ctx.JSONContext, typeConditions)
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

func ruleError(rule *kyverno.Rule, ruleType engineUtils.RuleType, msg string, err error) *response.RuleResponse {
	msg = fmt.Sprintf("%s: %s", msg, err.Error())
	return ruleResponse(rule, ruleType, msg, response.RuleStatusError)
}

func ruleResponse(rule *kyverno.Rule, ruleType engineUtils.RuleType, msg string, status response.RuleStatus) *response.RuleResponse {
	return &response.RuleResponse{
		Name:    rule.Name,
		Type:    ruleType.String(),
		Message: msg,
		Status:  status,
	}
}

func incrementAppliedCount(resp *response.EngineResponse) {
	resp.PolicyResponse.RulesAppliedCount++
}

func incrementErrorCount(resp *response.EngineResponse) {
	resp.PolicyResponse.RulesErrorCount++
}
