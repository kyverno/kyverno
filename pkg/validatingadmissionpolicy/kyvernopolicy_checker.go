package validatingadmissionpolicy

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/ext/wildcard"
)

// CanGenerateVAP check if Kyverno policy can be translated to a Kubernetes ValidatingAdmissionPolicy
func CanGenerateVAP(spec *kyvernov1.Spec) (bool, string) {
	var msg string
	if len(spec.Rules) > 1 {
		msg = "skip generating ValidatingAdmissionPolicy: multiple rules are not applicable."
		return false, msg
	}

	rule := spec.Rules[0]
	if !rule.HasValidateCEL() {
		msg = "skip generating ValidatingAdmissionPolicy for non CEL rules."
		return false, msg
	}

	if len(spec.ValidationFailureActionOverrides) > 1 {
		msg = "skip generating ValidatingAdmissionPolicy: multiple validationFailureActionOverrides are not applicable."
		return false, msg
	}

	if len(spec.ValidationFailureActionOverrides) != 0 && len(spec.ValidationFailureActionOverrides[0].Namespaces) != 0 {
		msg = "skip generating ValidatingAdmissionPolicy: Namespaces in validationFailureActionOverrides is not applicable."
		return false, msg
	}

	// check the matched/excluded resources of the CEL rule.
	match, exclude := rule.MatchResources, rule.ExcludeResources
	if !exclude.UserInfo.IsEmpty() || !exclude.ResourceDescription.IsEmpty() || exclude.All != nil || exclude.Any != nil {
		msg = "skip generating ValidatingAdmissionPolicy: Exclude is not applicable."
		return false, msg
	}
	if ok, msg := checkUserInfo(match.UserInfo); !ok {
		return false, msg
	}
	if ok, msg := checkResources(match.ResourceDescription); !ok {
		return false, msg
	}

	var (
		containsNamespaceSelector = false
		containsObjectSelector    = false
	)

	// since 'any' specify resources which will be ORed, it can be converted into multiple NamedRuleWithOperations in ValidatingAdmissionPolicy
	for _, value := range match.Any {
		if ok, msg := checkUserInfo(value.UserInfo); !ok {
			return false, msg
		}
		if ok, msg := checkResources(value.ResourceDescription); !ok {
			return false, msg
		}

		if value.NamespaceSelector != nil {
			containsNamespaceSelector = true
		}
		if value.Selector != nil {
			containsObjectSelector = true
		}
	}
	// since namespace/object selectors are applied to all NamedRuleWithOperations in ValidatingAdmissionPolicy, then
	// we can't have more than one resource with namespace/object selectors.
	if len(match.Any) > 1 && (containsNamespaceSelector || containsObjectSelector) {
		msg = "skip generating ValidatingAdmissionPolicy: NamespaceSelector / ObjectSelector across multiple resources are not applicable."
		return false, msg
	}

	// since 'all' specify resources which will be ANDed, we can't have more than one resource.
	if match.All != nil {
		if len(match.All) > 1 {
			msg = "skip generating ValidatingAdmissionPolicy: multiple 'all' is not applicable."
			return false, msg
		} else {
			if ok, msg := checkUserInfo(match.All[0].UserInfo); !ok {
				return false, msg
			}
			if ok, msg := checkResources(match.All[0].ResourceDescription); !ok {
				return false, msg
			}
		}
	}

	return true, msg
}

func checkResources(resource kyvernov1.ResourceDescription) (bool, string) {
	var msg string
	if len(resource.Annotations) != 0 {
		msg = "skip generating ValidatingAdmissionPolicy: Annotations in resource description is not applicable."
		return false, msg
	}
	if resource.Name != "" && wildcard.ContainsWildcard(resource.Name) {
		msg = "skip generating ValidatingAdmissionPolicy: wildcards in resource name is not applicable."
		return false, msg
	}
	for _, name := range resource.Names {
		if wildcard.ContainsWildcard(name) {
			msg = "skip generating ValidatingAdmissionPolicy: wildcards in resource name is not applicable."
			return false, msg
		}
	}
	for _, ns := range resource.Namespaces {
		if wildcard.ContainsWildcard(ns) {
			msg = "skip generating ValidatingAdmissionPolicy: wildcards in namespace name is not applicable."
			return false, msg
		}
	}
	return true, msg
}

func checkUserInfo(info kyvernov1.UserInfo) (bool, string) {
	var msg string
	if !info.IsEmpty() {
		msg = "skip generating ValidatingAdmissionPolicy: Roles / ClusterRoles / Subjects in `any/all` is not applicable."
		return false, msg
	}
	return true, msg
}
