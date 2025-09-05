package metrics

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
)

func ParsePolicyValidationMode(validationFailureAction kyvernov1.ValidationFailureAction) (PolicyValidationMode, error) {
	if validationFailureAction.Enforce() {
		return Enforce, nil
	}
	return Audit, nil
}

func ParsePolicyBackgroundMode(policy kyvernov1.PolicyInterface) PolicyBackgroundMode {
	if policy.BackgroundProcessingEnabled() {
		return BackgroundTrue
	}
	return BackgroundFalse
}

func ParseRuleType(rule kyvernov1.Rule) RuleType {
	if rule.Validation != nil && !datautils.DeepEqual(*rule.Validation, kyvernov1.Validation{}) {
		return Validate
	}
	if rule.Mutation != nil && !datautils.DeepEqual(*rule.Mutation, kyvernov1.Mutation{}) {
		return Mutate
	}
	if rule.Generation != nil && !datautils.DeepEqual(*rule.Generation, kyvernov1.Generation{}) {
		return Generate
	}
	if len(rule.VerifyImages) > 0 {
		return ImageVerify
	}
	return EmptyRuleType
}

func ParseResourceRequestOperation(requestOperationStr string) (ResourceRequestOperation, error) {
	switch requestOperationStr {
	case "CREATE":
		return ResourceCreated, nil
	case "UPDATE":
		return ResourceUpdated, nil
	case "DELETE":
		return ResourceDeleted, nil
	case "CONNECT":
		return ResourceConnected, nil
	default:
		return "", fmt.Errorf("unknown request operation made by resource: %s. Allowed requests: 'CREATE', 'UPDATE', 'DELETE', 'CONNECT'", requestOperationStr)
	}
}

func ParseRuleTypeFromEngineRuleResponse(rule engineapi.RuleResponse) RuleType {
	switch rule.RuleType() {
	case "Validation":
		return Validate
	case "Mutation":
		return Mutate
	case "Generation":
		return Generate
	case "ImageVerify":
		return ImageVerify
	default:
		return EmptyRuleType
	}
}

func GetPolicyInfos(policy kyvernov1.PolicyInterface) (string, string, PolicyType, PolicyBackgroundMode, PolicyValidationMode, error) {
	name := policy.GetName()
	namespace := ""
	policyType := Cluster
	if policy.IsNamespaced() {
		namespace = policy.GetNamespace()
		policyType = Namespaced
	}
	backgroundMode := ParsePolicyBackgroundMode(policy)
	isEnforce := policy.GetSpec().HasValidateEnforce()
	var validationMode PolicyValidationMode
	if isEnforce {
		validationMode = Enforce
	} else {
		validationMode = Audit
	}
	return name, namespace, policyType, backgroundMode, validationMode, nil
}
