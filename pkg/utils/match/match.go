package match

import (
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/engine/wildcards"
	"github.com/kyverno/kyverno/pkg/logging"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	"go.uber.org/multierr"
	"golang.org/x/exp/slices"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func CheckNamespace(statement string, resource unstructured.Unstructured) error {
	if statement == "" {
		return nil
	}
	if resource.GetNamespace() == statement {
		return nil
	}
	return fmt.Errorf("resource namespace (%s) doesn't match statement (%s)", resource.GetNamespace(), statement)
}

func CheckMatchesResources(
	resource unstructured.Unstructured,
	statement kyvernov2beta1.MatchResources,
	namespaceLabels map[string]string,
	subresourceGVKToAPIResource map[string]*metav1.APIResource,
	subresourceInAdmnReview string,
	admissionInfo kyvernov1beta1.RequestInfo,
	excludeGroupRole []string,
) error {
	var errs []error
	if len(statement.Any) > 0 {
		// include object if ANY of the criteria match
		// so if one matches then break from loop
		oneMatched := false
		for _, rmr := range statement.Any {
			// if there are no errors it means it was a match
			if len(checkResourceFilter(
				rmr,
				resource,
				namespaceLabels,
				subresourceGVKToAPIResource,
				subresourceInAdmnReview,
				admissionInfo,
				excludeGroupRole,
			)) == 0 {
				oneMatched = true
				break
			}
		}
		if !oneMatched {
			errs = append(errs, fmt.Errorf("no resource matched"))
		}
	} else if len(statement.All) > 0 {
		// include object if ALL of the criteria match
		for _, rmr := range statement.All {
			errs = append(
				errs,
				checkResourceFilter(
					rmr,
					resource,
					namespaceLabels,
					subresourceGVKToAPIResource,
					subresourceInAdmnReview,
					admissionInfo,
					excludeGroupRole,
				)...,
			)
		}
	}
	return multierr.Combine(errs...)
}

func checkResourceFilter(
	statement kyvernov1.ResourceFilter,
	resource unstructured.Unstructured,
	namespaceLabels map[string]string,
	subresourceGVKToAPIResource map[string]*metav1.APIResource,
	subresourceInAdmnReview string,
	admissionInfo kyvernov1beta1.RequestInfo,
	excludeGroupRole []string,
) []error {
	var errs []error
	// checking if the block is empty
	if statement.IsEmpty() {
		errs = append(errs, fmt.Errorf("statement cannot be empty"))
		return errs
	}
	matchErrs := checkResourceDescription(
		statement.ResourceDescription,
		resource,
		namespaceLabels,
		subresourceGVKToAPIResource,
		subresourceInAdmnReview,
	)
	userErrs := checkUserInfo(
		statement.UserInfo,
		admissionInfo,
		excludeGroupRole,
	)
	errs = append(errs, matchErrs...)
	errs = append(errs, userErrs...)
	return errs
}

func checkUserInfo(
	userInfo kyvernov1.UserInfo,
	admissionInfo kyvernov1beta1.RequestInfo,
	excludeGroupRole []string,
) []error {
	var errs []error
	var excludeKeys []string
	excludeKeys = append(excludeKeys, admissionInfo.AdmissionUserInfo.Groups...)
	excludeKeys = append(excludeKeys, admissionInfo.AdmissionUserInfo.Username)
	if len(userInfo.Roles) > 0 && !datautils.SliceContains(excludeKeys, excludeGroupRole...) {
		if !datautils.SliceContains(userInfo.Roles, admissionInfo.Roles...) {
			errs = append(errs, fmt.Errorf("user info does not match roles for the given conditionBlock"))
		}
	}
	if len(userInfo.ClusterRoles) > 0 && !datautils.SliceContains(excludeKeys, excludeGroupRole...) {
		if !datautils.SliceContains(userInfo.ClusterRoles, admissionInfo.ClusterRoles...) {
			errs = append(errs, fmt.Errorf("user info does not match clustersRoles for the given conditionBlock"))
		}
	}
	if len(userInfo.Subjects) > 0 {
		if !checkSubjects(userInfo.Subjects, admissionInfo.AdmissionUserInfo, excludeGroupRole) {
			errs = append(errs, fmt.Errorf("user info does not match subject for the given conditionBlock"))
		}
	}
	return errs
}

// matchSubjects return true if one of ruleSubjects exist in userInfo
func checkSubjects(
	ruleSubjects []rbacv1.Subject,
	userInfo authenticationv1.UserInfo,
	excludeGroupRole []string,
) bool {
	const SaPrefix = "system:serviceaccount:"
	userGroups := append(userInfo.Groups, userInfo.Username)
	// TODO: see issue https://github.com/kyverno/kyverno/issues/861
	for _, e := range excludeGroupRole {
		ruleSubjects = append(ruleSubjects, rbacv1.Subject{Kind: "Group", Name: e})
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

func checkResourceDescription(
	conditionBlock kyvernov1.ResourceDescription,
	resource unstructured.Unstructured,
	namespaceLabels map[string]string,
	subresourceGVKToAPIResource map[string]*metav1.APIResource,
	subresourceInAdmnReview string,
) []error {
	var errs []error
	if len(conditionBlock.Kinds) > 0 {
		// Matching on ephemeralcontainers even when they are not explicitly specified is only applicable to policies.
		if !CheckKind(subresourceGVKToAPIResource, conditionBlock.Kinds, resource.GroupVersionKind(), subresourceInAdmnReview, false) {
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
	return errs
}

// CheckKind checks if the resource kind matches the kinds in the policy. If the policy matches on subresources, then those resources are
// present in the subresourceGVKToAPIResource map. Set allowEphemeralContainers to true to allow ephemeral containers to be matched even when the
// policy does not explicitly match on ephemeral containers and only matches on pods.
func CheckKind(subresourceGVKToAPIResource map[string]*metav1.APIResource, kinds []string, gvk schema.GroupVersionKind, subresourceInAdmnReview string, allowEphemeralContainers bool) bool {
	result := false
	for _, k := range kinds {
		if k != "*" {
			gv, kind := kubeutils.GetKindFromGVK(k)
			apiResource, ok := subresourceGVKToAPIResource[k]
			if ok {
				result = apiResource.Group == gvk.Group && (apiResource.Version == gvk.Version || strings.Contains(gv, "*")) && apiResource.Kind == gvk.Kind
			} else { // if the kind is not found in the subresourceGVKToAPIResource, then it is not a subresource
				result = kind == gvk.Kind &&
					(subresourceInAdmnReview == "" ||
						(allowEphemeralContainers && subresourceInAdmnReview == "ephemeralcontainers"))
				if gv != "" {
					result = result && kubeutils.GroupVersionMatches(gv, gvk.GroupVersion().String())
				}
			}
		} else {
			result = true
		}

		if result {
			break
		}
	}
	return result
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
