package metrics

import (
	"fmt"
	"slices"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func parsePolicyBackgroundMode(policy kyvernov1.PolicyInterface) PolicyBackgroundMode {
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
	backgroundMode := parsePolicyBackgroundMode(policy)
	isEnforce := policy.GetSpec().HasValidateEnforce()
	var validationMode PolicyValidationMode
	if isEnforce {
		validationMode = Enforce
	} else {
		validationMode = Audit
	}
	return name, namespace, policyType, backgroundMode, validationMode, nil
}

func GetCELPolicyInfos(policy v1alpha1.GenericPolicy) (string, string, PolicyBackgroundMode, admissionregistrationv1.ValidationAction) {
	name := policy.GetName()
	validationMode := admissionregistrationv1.Audit
	backgroundMode := BackgroundFalse
	policyType := ""

	switch p := policy.(type) {
	case *v1alpha1.ValidatingPolicy:
		policyType = "Validating"

		if p.Spec.BackgroundEnabled() {
			backgroundMode = BackgroundTrue
		}
		if slices.Contains(p.Spec.ValidationActions(), admissionregistrationv1.Deny) {
			validationMode = admissionregistrationv1.Deny
		}
	case *v1alpha1.ImageValidatingPolicy:
		policyType = "ImageValidating"

		if p.Spec.BackgroundEnabled() {
			backgroundMode = BackgroundTrue
		}
		if slices.Contains(p.Spec.ValidationActions(), admissionregistrationv1.Deny) {
			validationMode = admissionregistrationv1.Deny
		}
	case *v1alpha1.MutatingPolicy:
		policyType = "Mutating"

		if p.Spec.BackgroundEnabled() {
			backgroundMode = BackgroundTrue
		}
	case *v1alpha1.GeneratingPolicy:
		policyType = "Generating"
	}

	return name, policyType, backgroundMode, validationMode
}
