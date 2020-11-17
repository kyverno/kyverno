package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/utils"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/minio/minio/pkg/wildcard"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/resourcecache"
	"k8s.io/apimachinery/pkg/runtime"
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

func checkAnnotations(annotations map[string]string, resourceAnnotations map[string]string) bool {
	for k, v := range annotations {
		if len(resourceAnnotations) == 0 {
			return false
		}
		if resourceAnnotations[k] != v {
			return false
		}
	}
	return true
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
// should be: OR (across & inside) attributes
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
func copyConditions(original []kyverno.Condition) []kyverno.Condition {
	var copy []kyverno.Condition
	for _, condition := range original {
		copy = append(copy, *condition.DeepCopy())
	}
	return copy
}

// excludeResource checks if the resource has ownerRef set
func excludeResource(resource unstructured.Unstructured) bool {
	kind := resource.GetKind()
	if kind == "Pod" || kind == "Job" {
		if len(resource.GetOwnerReferences()) > 0 {
			return true
		}
	}

	return false
}

// SkipPolicyApplication returns true:
// - if the policy has auto-gen annotation && resource == Pod
// - if the auto-gen contains cronJob && resource == Job
func SkipPolicyApplication(policy kyverno.ClusterPolicy, resource unstructured.Unstructured) bool {
	if policy.HasAutoGenAnnotation() && excludeResource(resource) {
		return true
	}

	if podControllers, ok := policy.GetAnnotations()[PodControllersAnnotation]; ok {
		if strings.Contains(podControllers, "CronJob") && excludeResource(resource) {
			return true
		}
	}

	return false
}

// AddResourceToContext - Add the Configmap JSON to Context.
// it will read configmaps (can be extended to get other type of resource like secrets, namespace etc)
// from the informer cache and add the configmap data to context
func AddResourceToContext(logger logr.Logger, contextEntries []kyverno.ContextEntry, resCache resourcecache.ResourceCacheIface, ctx *context.Context) error {
	if len(contextEntries) == 0 {
		return nil
	}

	// get GVR Cache for "configmaps"
	// can get cache for other resources if the informers are enabled in resource cache
	gvrC := resCache.GetGVRCache("configmaps")

	if gvrC != nil {
		lister := gvrC.GetLister()
		for _, context := range contextEntries {
			contextData := make(map[string]interface{})
			name := context.ConfigMap.Name
			namespace := context.ConfigMap.Namespace
			if namespace == "" {
				namespace = "default"
			}

			key := fmt.Sprintf("%s/%s", namespace, name)
			obj, err := lister.Get(key)
			if err != nil {
				logger.Error(err, fmt.Sprintf("failed to read configmap %s/%s from cache", namespace, name))
				continue
			}

			unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
			if err != nil {
				logger.Error(err, "failed to convert context runtime object to unstructured")
				continue
			}

			// extract configmap data
			contextData["data"] = unstructuredObj["data"]
			contextData["metadata"] = unstructuredObj["metadata"]
			contextNamedData := make(map[string]interface{})
			contextNamedData[context.Name] = contextData
			jdata, err := json.Marshal(contextNamedData)
			if err != nil {
				logger.Error(err, "failed to unmarshal context data")
				continue
			}

			// add data to context
			err = ctx.AddJSON(jdata)
			if err != nil {
				logger.Error(err, "failed to load context json")
				continue
			}
		}
		return nil
	}
	return errors.New("configmaps GVR Cache not found")
}
