package policyengine

import (
	"fmt"
	"log"

	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kube-policy/pkg/policyengine/mutation"
)

// Generate should be called to process generate rules on the resource
func Generate(logger *log.Logger, policy types.Policy, rawResource []byte) ([]GenerateReturnData, error) {
	patchingSets := mutation.GetPolicyPatchingSets(policy)
	generatedList := []GenerateReturnData{}
	for ruleIdx, rule := range policy.Spec.Rules {
		err := rule.Validate()
		if err != nil {
			logger.Printf("Invalid rule detected: #%d in policy %s, err: %v\n", ruleIdx, policy.ObjectMeta.Name, err)
			continue
		}
		if ok, err := mutation.IsRuleApplicableToResource(rawResource, rule.Resource); !ok {
			logger.Printf("Rule %d of policy %s is not applicable to the request", ruleIdx, policy.Name)
			return nil, err
		}
		resourceKind := mutation.ParseKindFromObject(rawResource)

		// configMapGenerator and secretGenerator can be applied only to namespaces
		if resourceKind == "Namespace" {
			generatedData, err := applyRuleGenerators(rawResource, rule)
			if err != nil && patchingSets == mutation.PatchingSetsStopOnError {
				return nil, fmt.Errorf("Failed to apply generators from rule #%d: %s", ruleIdx, err)
			}
			generatedList = append(generatedList, generatedData...)
		}
	}
	return generatedList, nil
}

// Applies "configMapGenerator" and "secretGenerator" described in PolicyRule
func applyRuleGenerators(rawResource []byte, rule types.PolicyRule) ([]GenerateReturnData, error) {
	returnData := []GenerateReturnData{}
	namespaceName := mutation.ParseNameFromObject(rawResource)
	var generator *types.PolicyConfigGenerator
	// Apply config map generator rule
	generator, err := applyConfigGenerator(rule.ConfigMapGenerator, namespaceName, "ConfigMap")
	if err != nil {
		return returnData, err
	}
	returnData = append(returnData, GenerateReturnData{namespaceName, "ConfigMap", *generator})
	// Apply secrets generator rule
	generator, err = applyConfigGenerator(rule.SecretGenerator, namespaceName, "Secret")
	if err != nil {
		return returnData, err
	}
	returnData = append(returnData, GenerateReturnData{namespaceName, "Secret", *generator})

	return returnData, nil
}

// Creates resourceKind (ConfigMap or Secret) with parameters specified in generator in cluster specified in request.
func applyConfigGenerator(generator *types.PolicyConfigGenerator, namespace string, configKind string) (*types.PolicyConfigGenerator, error) {
	if generator == nil {
		return nil, nil
	}
	err := generator.Validate()
	if err != nil {
		return nil, fmt.Errorf("Generator for '%s' is invalid: %s", configKind, err)
	}
	switch configKind {
	case "ConfigMap":
		return generator, nil
		//		err = kubeClient.GenerateConfigMap(*generator, namespace)
	case "Secret":
		return generator, nil
	default:
		return nil, fmt.Errorf("Unsupported config Kind '%s'", configKind)
	}
}

//GenerateReturnData holds the generator details
type GenerateReturnData struct {
	namespace  string
	configKind string
	generator  types.PolicyConfigGenerator
}
