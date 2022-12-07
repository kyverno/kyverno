package cleanup

import (
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/engine/wildcards"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	"go.uber.org/multierr"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func checkNamespace(statement string, resource unstructured.Unstructured) error {
	if statement == "" {
		return nil
	}
	if resource.GetNamespace() == statement {
		return nil
	}
	return fmt.Errorf("resource namespace (%s) doesn't match statement (%s)", resource.GetNamespace(), statement)
}

func checkMatchesResources(
	resource unstructured.Unstructured,
	statement kyvernov2beta1.MatchResources,
	// ruleRef kyvernov1.Rule,
	//  admissionInfoRef kyvernov1beta1.RequestInfo,
	dynamicConfig []string,
	namespaceLabels map[string]string,
	// policyNamespace string,
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
				//  admissionInfo,
				resource,
				dynamicConfig,
				namespaceLabels,
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
					// admissionInfo,
					resource,
					dynamicConfig,
					namespaceLabels,
				)...,
			)
		}
	}
	return multierr.Combine(errs...)
}

func checkResourceFilter(
	statement kyvernov1.ResourceFilter,
	// admissionInfo kyvernov1beta1.RequestInfo,
	resource unstructured.Unstructured,
	dynamicConfig []string,
	namespaceLabels map[string]string,
) []error {
	var errs []error
	// if reflect.DeepEqual(admissionInfo, kyvernov1.RequestInfo{}) {
	// 	rmr.UserInfo = kyvernov1.UserInfo{}
	// }
	// checking if the block is empty
	if statement.IsEmpty() {
		errs = append(errs, fmt.Errorf("statement cannot be empty"))
		return errs
	}
	matchErrs := checkResourceDescription(
		statement.ResourceDescription,
		/* rmr.UserInfo,*/ /*admissionInfo,*/
		resource,
		dynamicConfig,
		namespaceLabels,
	)
	errs = append(errs, matchErrs...)
	return errs
}

func checkResourceDescription(
	conditionBlock kyvernov1.ResourceDescription,
	resource unstructured.Unstructured,
	dynamicConfig []string,
	namespaceLabels map[string]string,
) []error {
	var errs []error
	if len(conditionBlock.Kinds) > 0 {
		if !checkKind(conditionBlock.Kinds, resource.GetKind(), resource.GroupVersionKind()) {
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
	// keys := append(admissionInfo.AdmissionUserInfo.Groups, admissionInfo.AdmissionUserInfo.Username)
	// var userInfoErrors []error
	// if len(userInfo.Roles) > 0 && !utils.SliceContains(keys, dynamicConfig...) {
	// 	if !utils.SliceContains(userInfo.Roles, admissionInfo.Roles...) {
	// 		userInfoErrors = append(userInfoErrors, fmt.Errorf("user info does not match roles for the given conditionBlock"))
	// 	}
	// }

	// if len(userInfo.ClusterRoles) > 0 && !utils.SliceContains(keys, dynamicConfig...) {
	// 	if !utils.SliceContains(userInfo.ClusterRoles, admissionInfo.ClusterRoles...) {
	// 		userInfoErrors = append(userInfoErrors, fmt.Errorf("user info does not match clustersRoles for the given conditionBlock"))
	// 	}
	// }

	// if len(userInfo.Subjects) > 0 {
	// 	if !matchSubjects(userInfo.Subjects, admissionInfo.AdmissionUserInfo, dynamicConfig) {
	// 		userInfoErrors = append(userInfoErrors, fmt.Errorf("user info does not match subject for the given conditionBlock"))
	// 	}
	// }
	// return append(errs, userInfoErrors...)
	return errs
}

func checkKind(kinds []string, resourceKind string, gvk schema.GroupVersionKind) bool {
	title := cases.Title(language.Und, cases.NoLower)
	for _, k := range kinds {
		parts := strings.Split(k, "/")
		if len(parts) == 1 {
			if k == "*" || resourceKind == title.String(k) {
				return true
			}
		}
		if len(parts) == 2 {
			kindParts := strings.SplitN(parts[1], ".", 2)
			if gvk.Kind == title.String(kindParts[0]) && gvk.Version == parts[0] {
				return true
			}
		}
		if len(parts) == 3 || len(parts) == 4 {
			kindParts := strings.SplitN(parts[2], ".", 2)
			if gvk.Group == parts[0] && (gvk.Version == parts[1] || parts[1] == "*") && gvk.Kind == title.String(kindParts[0]) {
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
