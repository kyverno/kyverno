package utils

import (
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/utils/conditions"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	matched "github.com/kyverno/kyverno/pkg/utils/match"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// MatchesException takes a list of exceptions and checks if there is an exception applies to the incoming resource.
// It returns the matched policy exception.
func MatchesException(polexs []*kyvernov2.PolicyException, policyContext engineapi.PolicyContext, logger logr.Logger) []kyvernov2.PolicyException {
	var matchedExceptions []kyvernov2.PolicyException
	gvk, subresource := policyContext.ResourceKind()
	resource := policyContext.NewResource()
	if resource.Object == nil {
		resource = policyContext.OldResource()
	}
	for _, polex := range polexs {
		match := checkMatchesResources(
			resource,
			polex.Spec.Match,
			policyContext.NamespaceLabels(),
			policyContext.AdmissionInfo(),
			gvk,
			subresource,
		)
		if match {
			if polex.Spec.Conditions != nil {
				passed, err := conditions.CheckAnyAllConditions(logger, policyContext.JSONContext(), *polex.Spec.Conditions)
				if err != nil {
					return nil
				}
				if !passed {
					continue
				}
			}
			matchedExceptions = append(matchedExceptions, *polex)
		}
	}
	return matchedExceptions
}

func checkMatchesResources(
	resource unstructured.Unstructured,
	statement kyvernov2beta1.MatchResources,
	namespaceLabels map[string]string,
	admissionInfo kyvernov2.RequestInfo,
	gvk schema.GroupVersionKind,
	subresource string,
) bool {
	if len(statement.Any) > 0 {
		for _, rmr := range statement.Any {
			if checkResourceFilter(rmr, resource, namespaceLabels, admissionInfo, gvk, subresource) {
				return true
			}
		}
		return false
	} else if len(statement.All) > 0 {
		for _, rmr := range statement.All {
			if !checkResourceFilter(rmr, resource, namespaceLabels, admissionInfo, gvk, subresource) {
				return false
			}
		}
		return true
	}
	return false
}

func checkResourceFilter(
	statement kyvernov1.ResourceFilter,
	resource unstructured.Unstructured,
	namespaceLabels map[string]string,
	admissionInfo kyvernov2.RequestInfo,
	gvk schema.GroupVersionKind,
	subresource string,
) bool {
	if statement.IsEmpty() {
		return false
	}
	return checkResourceDescription(statement.ResourceDescription, resource, namespaceLabels, gvk, subresource) &&
		checkUserInfo(statement.UserInfo, admissionInfo)
}

func checkResourceDescription(
	conditionBlock kyvernov1.ResourceDescription,
	resource unstructured.Unstructured,
	namespaceLabels map[string]string,
	gvk schema.GroupVersionKind,
	subresource string,
) bool {
	if len(conditionBlock.Kinds) > 0 {
		if !matched.CheckKind(conditionBlock.Kinds, gvk, subresource, true) {
			return false
		}
	}
	if conditionBlock.Name != "" || len(conditionBlock.Names) > 0 {
		resourceName := resource.GetName()
		if resourceName == "" {
			resourceName = resource.GetGenerateName()
		}
		if conditionBlock.Name != "" {
			if !matched.CheckName(conditionBlock.Name, resourceName) {
				return false
			}
		}
		if len(conditionBlock.Names) > 0 {
			noneMatch := true
			for i := range conditionBlock.Names {
				if matched.CheckName(conditionBlock.Names[i], resourceName) {
					noneMatch = false
					break
				}
			}
			if noneMatch {
				return false
			}
		}
	}
	if len(conditionBlock.Namespaces) > 0 {
		if !matched.CheckNameSpace(conditionBlock.Namespaces, resource) {
			return false
		}
	}
	if len(conditionBlock.Annotations) > 0 {
		if !matched.CheckAnnotations(conditionBlock.Annotations, resource.GetAnnotations()) {
			return false
		}
	}
	if conditionBlock.Selector != nil {
		hasPassed, err := matched.CheckSelector(conditionBlock.Selector, resource.GetLabels())
		if err != nil {
			return false
		} else {
			if !hasPassed {
				return false
			}
		}
	}
	if conditionBlock.NamespaceSelector != nil && resource.GetKind() != "Namespace" && resource.GetKind() != "" {
		hasPassed, err := matched.CheckSelector(conditionBlock.NamespaceSelector, namespaceLabels)
		if err != nil {
			return false
		} else {
			if !hasPassed {
				return false
			}
		}
	}
	return true
}

func checkUserInfo(userInfo kyvernov1.UserInfo, admissionInfo kyvernov2.RequestInfo) bool {
	if len(userInfo.Roles) > 0 {
		if !datautils.SliceContains(userInfo.Roles, admissionInfo.Roles...) {
			return false
		}
	}
	if len(userInfo.ClusterRoles) > 0 {
		if !datautils.SliceContains(userInfo.ClusterRoles, admissionInfo.ClusterRoles...) {
			return false
		}
	}
	if len(userInfo.Subjects) > 0 {
		if !matched.CheckSubjects(userInfo.Subjects, admissionInfo.AdmissionUserInfo) {
			return false
		}
	}
	return true
}
