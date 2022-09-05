package metrics

import (
	"fmt"
	"reflect"

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/engine/response"
)

func ParsePolicyValidationMode(validationFailureAction kyvernov2beta1.ValidationFailureAction) (PolicyValidationMode, error) {
	switch validationFailureAction {
	case kyvernov2beta1.Enforce:
		return Enforce, nil
	case kyvernov2beta1.Audit:
		return Audit, nil
	default:
		return "", fmt.Errorf("wrong validation failure action found %s. Allowed: '%s', '%s'", validationFailureAction, "enforce", "audit")
	}
}

func ParsePolicyBackgroundMode(policy kyvernov2beta1.PolicyInterface) PolicyBackgroundMode {
	if policy.BackgroundProcessingEnabled() {
		return BackgroundTrue
	}
	return BackgroundFalse
}

func ParseRuleType(rule kyvernov2beta1.Rule) RuleType {
	if !reflect.DeepEqual(rule.Validation, kyvernov2beta1.Validation{}) {
		return Validate
	}
	if !reflect.DeepEqual(rule.Mutation, kyvernov2beta1.Mutation{}) {
		return Mutate
	}
	if !reflect.DeepEqual(rule.Generation, kyvernov2beta1.Generation{}) {
		return Generate
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

func ParseRuleTypeFromEngineRuleResponse(rule response.RuleResponse) RuleType {
	switch rule.Type {
	case "Validation":
		return Validate
	case "Mutation":
		return Mutate
	case "Generation":
		return Generate
	default:
		return EmptyRuleType
	}
}

func GetPolicyInfos(policy kyvernov2beta1.PolicyInterface) (string, string, PolicyType, PolicyBackgroundMode, PolicyValidationMode, error) {
	name := policy.GetName()
	namespace := ""
	policyType := Cluster
	if policy.IsNamespaced() {
		namespace = policy.GetNamespace()
		policyType = Namespaced
	}
	backgroundMode := ParsePolicyBackgroundMode(policy)
	validationMode, err := ParsePolicyValidationMode(policy.GetSpec().GetValidationFailureAction())
	return name, namespace, policyType, backgroundMode, validationMode, err
}
