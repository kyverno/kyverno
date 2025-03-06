package admissionpolicy

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/ext/wildcard"
)

// CanGenerateVAP check if Kyverno policy and a PolicyException can be translated to a Kubernetes ValidatingAdmissionPolicy
func CanGenerateVAP(spec *kyvernov1.Spec, exceptions []kyvernov2.PolicyException, validate bool) (bool, string) {
	var msg string
	if ok, msg := checkPolicy(spec, validate); !ok {
		return false, msg
	}

	if ok, msg := checkExceptions(exceptions); !ok {
		return false, msg
	}

	return true, msg
}

func checkExceptions(exceptions []kyvernov2.PolicyException) (bool, string) {
	var msg string
	for _, exception := range exceptions {
		spec := exception.Spec
		for _, exception := range spec.Exceptions {
			if len(exception.RuleNames) > 1 {
				msg = "skip generating ValidatingAdmissionPolicy: multiple ruleNames in PolicyException is not applicable."
				return false, msg
			}
		}

		if spec.Conditions != nil {
			msg = "skip generating ValidatingAdmissionPolicy: Conditions in PolicyException is not applicable."
			return false, msg
		}

		exclude := spec.Match
		if ok, msg := checkResourceFilter(exclude.Any, false); !ok {
			return false, msg
		}

		if len(exclude.All) > 1 {
			msg = "skip generating ValidatingAdmissionPolicy: multiple 'all' in the PolicyException's match block is not applicable."
			return false, msg
		}
		if ok, msg := checkResourceFilter(exclude.All, false); !ok {
			return false, msg
		}
	}
	return true, msg
}

func checkPolicy(spec *kyvernov1.Spec, validate bool) (bool, string) {
	var msg string
	if ok, msg := checkRuleCount(spec); !ok {
		return false, msg
	}

	rule := spec.Rules[0]
	if ok, msg := checkRuleType(rule, validate); !ok {
		return false, msg
	}

	if ok, msg := checkValidationFailureActionOverrides(spec.ValidationFailureActionOverrides); !ok {
		return false, msg
	}

	if ok, msg := checkValidationFailureActionOverrides(rule.Validation.FailureActionOverrides); !ok {
		return false, msg
	}

	// check the matched/excluded resources of the CEL rule.
	match := rule.MatchResources
	if ok, msg := checkUserInfo(match.UserInfo); !ok {
		return false, msg
	}
	if ok, msg := checkResources(match.ResourceDescription, true); !ok {
		return false, msg
	}
	if ok, msg := checkResourceFilter(match.Any, true); !ok {
		return false, msg
	}
	if len(match.All) > 1 {
		msg = "skip generating ValidatingAdmissionPolicy: multiple 'all' in the match block is not applicable."
		return false, msg
	}
	if ok, msg := checkResourceFilter(match.All, true); !ok {
		return false, msg
	}
	if rule.ExcludeResources != nil {
		exclude := rule.ExcludeResources
		if ok, msg := checkUserInfo(exclude.UserInfo); !ok {
			return false, msg
		}
		if ok, msg := checkResources(exclude.ResourceDescription, false); !ok {
			return false, msg
		}
		if ok, msg := checkResourceFilter(exclude.Any, false); !ok {
			return false, msg
		}
		if len(exclude.All) > 1 {
			msg = "skip generating ValidatingAdmissionPolicy: multiple 'all' in the exclude block is not applicable."
			return false, msg
		}
		if ok, msg := checkResourceFilter(exclude.All, false); !ok {
			return false, msg
		}
	}

	return true, msg
}

func checkRuleCount(spec *kyvernov1.Spec) (bool, string) {
	var msg string
	if len(spec.Rules) == 0 {
		msg = "skip generating ValidatingAdmissionPolicy: no rules found."
		return false, msg
	}
	if len(spec.Rules) > 1 {
		msg = "skip generating ValidatingAdmissionPolicy: multiple rules are not applicable."
		return false, msg
	}
	return true, msg
}

func checkRuleType(rule kyvernov1.Rule, validate bool) (bool, string) {
	var msg string
	if !rule.HasValidateCEL() {
		msg = "skip generating ValidatingAdmissionPolicy for non CEL rules."
		return false, msg
	} else if !validate {
		if !rule.Validation.CEL.GenerateVAP() {
			msg = "skip generating ValidatingAdmissionPolicy: validate.cel.generate is not set to true."
			return false, msg
		}
	}
	return true, msg
}

func checkResources(resource kyvernov1.ResourceDescription, isMatch bool) (bool, string) {
	var msg string
	if !isMatch {
		if len(resource.Kinds) != 0 && len(resource.Namespaces) != 0 {
			msg = "skip generating ValidatingAdmissionPolicy: excluding a resource within a namespace is not applicable."
			return false, msg
		}
	}

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

func checkResourceFilter(resFilters kyvernov1.ResourceFilters, isMatch bool) (bool, string) {
	var msg string
	containsNamespaceSelector := false
	containsObjectSelector := false

	for _, value := range resFilters {
		if ok, msg := checkUserInfo(value.UserInfo); !ok {
			return false, msg
		}
		if ok, msg := checkResources(value.ResourceDescription, isMatch); !ok {
			return false, msg
		}

		if value.NamespaceSelector != nil {
			containsNamespaceSelector = true
		}
		if value.Selector != nil {
			containsObjectSelector = true
		}
	}

	if !isMatch {
		if containsNamespaceSelector || containsObjectSelector {
			msg = "skip generating ValidatingAdmissionPolicy: NamespaceSelector / ObjectSelector in the exclude block is not applicable."
			return false, msg
		}
	} else {
		if len(resFilters) > 1 && (containsNamespaceSelector || containsObjectSelector) {
			return false, "skip generating ValidatingAdmissionPolicy: NamespaceSelector / ObjectSelector across multiple resources in the match block are not applicable."
		}
	}

	return true, msg
}

func checkValidationFailureActionOverrides(validationFailureActionOverrides []kyvernov1.ValidationFailureActionOverride) (bool, string) {
	var msg string
	if len(validationFailureActionOverrides) > 1 {
		msg = "skip generating ValidatingAdmissionPolicy: multiple validationFailureActionOverrides are not applicable."
		return false, msg
	}

	if len(validationFailureActionOverrides) != 0 && len(validationFailureActionOverrides[0].Namespaces) != 0 {
		msg = "skip generating ValidatingAdmissionPolicy: Namespaces in validationFailureActionOverrides is not applicable."
		return false, msg
	}
	return true, msg
}
