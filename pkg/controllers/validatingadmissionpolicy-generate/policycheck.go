package validatingadmissionpolicygenerate

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

func checkResources(resources ...kyvernov1.ResourceDescription) bool {
	for _, res := range resources {
		if len(res.Namespaces) != 0 || len(res.Annotations) != 0 {
			debug.Info("skip generating validating admission policies: Namespaces / Annotations in resource description isn't applicable.")
			return false
		}
	}
	return true
}

func checkUserInfo(userInfos ...kyvernov1.UserInfo) bool {
	for _, info := range userInfos {
		if !info.IsEmpty() {
			debug.Info("skip generating validating admission policies: Roles / ClusterRoles / Subjects in `any/all` isn't applicable.")
			return false
		}
	}
	return true
}

func canGenerateVAP(spec *kyvernov1.Spec) bool {
	if len(spec.Rules) > 1 {
		debug.Info("skip generating validating admission policies: multiple rules aren't applicable.")
		return false
	}

	// check the common policy settings that apply to all rules.
	if !spec.HasValidate() {
		debug.Info("skip generating validating admission policies for non validate rules.")
		return false
	}

	rule := spec.Rules[0]
	if !rule.HasValidateCEL() {
		debug.Info("skip generating validating admission policies for non CEL rules.")
		return false
	}

	if len(spec.ValidationFailureActionOverrides) > 1 {
		debug.Info("skip generating validating admission policies: multiple validationFailureActionOverrides aren't applicable.")
		return false
	}

	if len(spec.ValidationFailureActionOverrides) != 0 && len(spec.ValidationFailureActionOverrides[0].Namespaces) != 0 {
		debug.Info("skip generating validating admission policies: Namespaces in validationFailureActionOverrides isn't applicable.")
		return false
	}

	// check the matched/excluded resources of the CEL rule.
	match, exclude := rule.MatchResources, rule.ExcludeResources
	if !checkUserInfo(match.UserInfo, exclude.UserInfo) {
		return false
	}
	if !checkResources(match.ResourceDescription, exclude.ResourceDescription) {
		return false
	}

	var (
		containsNamespaceSelector = false
		containsObjectSelector    = false
	)

	// since 'any' specify resources which will be ORed, it can be converted into multiple NamedRuleWithOperations in validating admission policy
	for _, value := range match.Any {
		if !checkUserInfo(value.UserInfo) {
			return false
		}
		if !checkResources(value.ResourceDescription) {
			return false
		}

		// since namespace/object selectors are applied to all NamedRuleWithOperations in validating admission policies, then
		// multiple namespace/object selectors aren't applicable across the `any` clause.
		if value.NamespaceSelector != nil {
			if containsNamespaceSelector {
				debug.Info("skip generating validating admission policies: multiple NamespaceSelector across 'any' aren't applicable.")
				return false
			}
			containsNamespaceSelector = true
		}
		if value.Selector != nil {
			if containsObjectSelector {
				debug.Info("skip generating validating admission policies: multiple ObjectSelector across 'any' aren't applicable.")
				return false
			}
			containsObjectSelector = true
		}
	}
	// since 'all' specify resources which will be ANDed, we can't have more than one resource.
	if match.All != nil {
		if len(match.All) > 1 {
			debug.Info("skip generating validating admission policies: multiple 'all' isn't applicable.")
			return false
		} else {
			if !checkUserInfo(match.All[0].UserInfo) {
				return false
			}
			if !checkResources(match.All[0].ResourceDescription) {
				return false
			}
		}
	}

	// since 'any' specify resources which will be ORed, it can be converted into multiple NamedRuleWithOperations in validating admission policy
	for _, value := range exclude.Any {
		if !checkUserInfo(value.UserInfo) {
			return false
		}
		if !checkResources(value.ResourceDescription) {
			return false
		}

		// since namespace/object selectors are applied to all NamedRuleWithOperations in validating admission policies, then
		// multiple namespace/object selectors aren't applicable across the `any` clause.
		if value.NamespaceSelector != nil {
			if containsNamespaceSelector {
				debug.Info("skip generating validating admission policies: multiple NamespaceSelector across 'any' aren't applicable.")
				return false
			}
			containsNamespaceSelector = true
		}
		if value.Selector != nil {
			if containsObjectSelector {
				debug.Info("skip generating validating admission policies: multiple ObjectSelector across 'any' aren't applicable.")
				return false
			}
			containsObjectSelector = true
		}
	}
	// since 'all' specify resources which will be ANDed, we can't have more than one resource.
	if exclude.All != nil {
		if len(exclude.All) > 1 {
			debug.Info("skip generating validating admission policies: multiple 'all' isn't applicable.")
			return false
		} else {
			if !checkUserInfo(exclude.All[0].UserInfo) {
				return false
			}
			if !checkResources(exclude.All[0].ResourceDescription) {
				return false
			}
		}
	}
	return true
}
