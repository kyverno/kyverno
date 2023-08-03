package vapgeneration

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

	// check the matched resources of the CEL rule.
	match, exclude := rule.MatchResources, rule.ExcludeResources
	if !checkUserInfo(match.UserInfo, exclude.UserInfo) {
		return false
	}
	if !checkResources(match.ResourceDescription, exclude.ResourceDescription) {
		return false
	}

	for _, value := range match.Any {
		if !checkUserInfo(value.UserInfo) {
			return false
		}
		if !checkResources(value.ResourceDescription) {
			return false
		}
	}
	for _, value := range match.All {
		if !checkUserInfo(value.UserInfo) {
			return false
		}
		if !checkResources(value.ResourceDescription) {
			return false
		}
	}
	for _, value := range exclude.Any {
		if !checkUserInfo(value.UserInfo) {
			return false
		}
		if !checkResources(value.ResourceDescription) {
			return false
		}
	}
	for _, value := range exclude.All {
		if !checkUserInfo(value.UserInfo) {
			return false
		}
		if !checkResources(value.ResourceDescription) {
			return false
		}
	}
	return true
}
