package match

import (
	"errors"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/ext/wildcard"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	admissionInfo kyvernov1beta1.RequestInfo,
	gvk schema.GroupVersionKind,
	subresource string,
) error {
	if len(statement.Any) > 0 {
		// include object if ANY of the criteria match
		// so if one matches then break from loop
		oneMatched := false
		for i := range statement.Any {
			// if there are no errors it means it was a match
			if err := checkResourceFilter(
				statement.Any[i],
				resource,
				namespaceLabels,
				admissionInfo,
				gvk,
				subresource,
			); err == nil {
				oneMatched = true
				break
			}
		}
		if !oneMatched {
			return errors.New("no resource matched")
		}
	} else if len(statement.All) > 0 {
		// include object if ALL of the criteria match
		for i := range statement.All {
			if err := checkResourceFilter(
				statement.All[i],
				resource,
				namespaceLabels,
				admissionInfo,
				gvk,
				subresource,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func checkResourceFilter(
	statement kyvernov1.ResourceFilter,
	resource unstructured.Unstructured,
	namespaceLabels map[string]string,
	admissionInfo kyvernov1beta1.RequestInfo,
	gvk schema.GroupVersionKind,
	subresource string,
) error {
	// checking if the block is empty
	if statement.IsEmpty() {
		return errors.New("statement cannot be empty")
	}

	if userErr := checkUserInfo(
		statement.UserInfo,
		admissionInfo,
	); userErr != nil {
		return userErr
	}

	if matchErr := checkResourceDescription(
		statement.ResourceDescription,
		resource,
		namespaceLabels,
		gvk,
		subresource,
	); matchErr != nil {
		return matchErr
	}

	return nil
}

func checkUserInfo(
	userInfo kyvernov1.UserInfo,
	admissionInfo kyvernov1beta1.RequestInfo,
) error {
	if len(userInfo.Roles) > 0 {
		if !datautils.SliceContains(userInfo.Roles, admissionInfo.Roles...) {
			return errors.New("user info does not match roles for the given conditionBlock")
		}
	}

	if len(userInfo.ClusterRoles) > 0 {
		if !datautils.SliceContains(userInfo.ClusterRoles, admissionInfo.ClusterRoles...) {
			return errors.New("user info does not match clustersRoles for the given conditionBlock")
		}
	}

	if len(userInfo.Subjects) > 0 {
		if !CheckSubjects(userInfo.Subjects, admissionInfo.AdmissionUserInfo) {
			return errors.New("user info does not match subject for the given conditionBlock")
		}
	}

	return nil
}

func checkResourceDescription(
	conditionBlock kyvernov1.ResourceDescription,
	resource unstructured.Unstructured,
	namespaceLabels map[string]string,
	gvk schema.GroupVersionKind,
	subresource string,
) error {
	if len(conditionBlock.Kinds) > 0 {
		// Matching on ephemeralcontainers even when they are not explicitly specified is only applicable to policies.
		if !CheckKind(conditionBlock.Kinds, gvk, subresource, false) {
			return fmt.Errorf("kind does not match %v", conditionBlock.Kinds)
		}
	}

	resourceName := resource.GetName()
	if resourceName == "" {
		resourceName = resource.GetGenerateName()
	}

	if conditionBlock.Name != "" {
		if !CheckName(conditionBlock.Name, resourceName) {
			return errors.New("name does not match")
		}
	}

	if len(conditionBlock.Names) > 0 {
		noneMatch := true
		for i := range conditionBlock.Names {
			if CheckName(conditionBlock.Names[i], resourceName) {
				noneMatch = false
				break
			}
		}
		if noneMatch {
			return errors.New("none of the names match")
		}
	}

	if len(conditionBlock.Namespaces) > 0 {
		if !checkNameSpace(conditionBlock.Namespaces, resource) {
			return errors.New("namespace does not match")
		}
	}

	if len(conditionBlock.Annotations) > 0 {
		if !CheckAnnotations(conditionBlock.Annotations, resource.GetAnnotations()) {
			return errors.New("annotations does not match")
		}
	}

	if conditionBlock.Selector != nil {
		hasPassed, err := CheckSelector(conditionBlock.Selector, resource.GetLabels())
		if err != nil {
			return fmt.Errorf("failed to parse selector: %v", err)
		} else {
			if !hasPassed {
				return errors.New("selector does not match")
			}
		}
	}

	if conditionBlock.NamespaceSelector != nil && resource.GetKind() != "Namespace" && resource.GetKind() != "" {
		hasPassed, err := CheckSelector(conditionBlock.NamespaceSelector, namespaceLabels)
		if err != nil {
			return fmt.Errorf("failed to parse namespace selector: %v", err)
		} else {
			if !hasPassed {
				return errors.New("namespace selector does not match")
			}
		}
	}

	return nil
}

func checkNameSpace(namespaces []string, resource unstructured.Unstructured) bool {
	resourceNameSpace := resource.GetNamespace()
	if resource.GetKind() == "Namespace" {
		resourceNameSpace = resource.GetName()
	}
	for i := range namespaces {
		if wildcard.Match(namespaces[i], resourceNameSpace) {
			return true
		}
	}
	return false
}
