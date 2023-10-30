package validatingadmissionpolicygenerate

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

func checkResources(resource kyvernov1.ResourceDescription) (bool, string) {
	var msg string
	if len(resource.Namespaces) != 0 || len(resource.Annotations) != 0 {
		msg = "skip generating ValidatingAdmissionPolicy: Namespaces / Annotations in resource description isn't applicable."
		return false, msg
	}
	return true, msg
}

func checkUserInfo(info kyvernov1.UserInfo) (bool, string) {
	var msg string
	if !info.IsEmpty() {
		msg = "skip generating ValidatingAdmissionPolicy: Roles / ClusterRoles / Subjects in `any/all` isn't applicable."
		return false, msg
	}
	return true, msg
}

func canGenerateVAP(spec *kyvernov1.Spec) (bool, string) {
	var msg string
	if len(spec.Rules) > 1 {
		msg = "skip generating ValidatingAdmissionPolicy: multiple rules aren't applicable."
		return false, msg
	}

	rule := spec.Rules[0]
	if !rule.HasValidateCEL() {
		msg = "skip generating ValidatingAdmissionPolicy for non CEL rules."
		return false, msg
	}

	if len(spec.ValidationFailureActionOverrides) > 1 {
		msg = "skip generating ValidatingAdmissionPolicy: multiple validationFailureActionOverrides aren't applicable."
		return false, msg
	}

	if len(spec.ValidationFailureActionOverrides) != 0 && len(spec.ValidationFailureActionOverrides[0].Namespaces) != 0 {
		msg = "skip generating ValidatingAdmissionPolicy: Namespaces in validationFailureActionOverrides isn't applicable."
		return false, msg
	}

	// check the matched/excluded resources of the CEL rule.
	match, exclude := rule.MatchResources, rule.ExcludeResources
	if !exclude.UserInfo.IsEmpty() || !exclude.ResourceDescription.IsEmpty() || exclude.All != nil || exclude.Any != nil {
		msg = "skip generating ValidatingAdmissionPolicy: Exclude isn't applicable."
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

		// since namespace/object selectors are applied to all NamedRuleWithOperations in ValidatingAdmissionPolicy, then
		// multiple namespace/object selectors aren't applicable across the `any` clause.
		if value.NamespaceSelector != nil {
			if containsNamespaceSelector {
				msg = "skip generating ValidatingAdmissionPolicy: multiple NamespaceSelector across 'any' aren't applicable."
				return false, msg
			}
			containsNamespaceSelector = true
		}
		if value.Selector != nil {
			if containsObjectSelector {
				msg = "skip generating ValidatingAdmissionPolicy: multiple ObjectSelector across 'any' aren't applicable."
				return false, msg
			}
			containsObjectSelector = true
		}
	}
	// since 'all' specify resources which will be ANDed, we can't have more than one resource.
	if match.All != nil {
		if len(match.All) > 1 {
			msg = "skip generating ValidatingAdmissionPolicy: multiple 'all' isn't applicable."
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

	// since 'any' specify resources which will be ORed, it can be converted into multiple NamedRuleWithOperations in ValidatingAdmissionPolicy
	for _, value := range exclude.Any {
		if ok, msg := checkUserInfo(value.UserInfo); !ok {
			return false, msg
		}
		if ok, msg := checkResources(value.ResourceDescription); !ok {
			return false, msg
		}

		// since namespace/object selectors are applied to all NamedRuleWithOperations in ValidatingAdmissionPolicy, then
		// multiple namespace/object selectors aren't applicable across the `any` clause.
		if value.NamespaceSelector != nil {
			if containsNamespaceSelector {
				msg = "skip generating ValidatingAdmissionPolicy: multiple NamespaceSelector across 'any' aren't applicable."
				return false, msg
			}
			containsNamespaceSelector = true
		}
		if value.Selector != nil {
			if containsObjectSelector {
				msg = "skip generating ValidatingAdmissionPolicy: multiple ObjectSelector across 'any' aren't applicable."
				return false, msg
			}
			containsObjectSelector = true
		}
	}
	// since 'all' specify resources which will be ANDed, we can't have more than one resource.
	if exclude.All != nil {
		if len(exclude.All) > 1 {
			msg = "skip generating ValidatingAdmissionPolicy: multiple 'all' isn't applicable."
			return false, msg
		} else {
			if ok, msg := checkUserInfo(exclude.All[0].UserInfo); !ok {
				return false, msg
			}
			if ok, msg := checkResources(exclude.All[0].ResourceDescription); !ok {
				return false, msg
			}
		}
	}
	return true, msg
}
