package policyengine

import (
	"errors"
	"fmt"

	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kube-policy/pkg/policyengine/mutation"
)

func (p *policyEngine) ProcessMutation(policy types.Policy, rawResource []byte) ([]mutation.PatchBytes, error) {
	patchingSets := mutation.GetPolicyPatchingSets(policy)
	var policyPatches []mutation.PatchBytes

	for ruleIdx, rule := range policy.Spec.Rules {
		err := rule.Validate()
		if err != nil {
			p.logger.Printf("Invalid rule detected: #%s in policy %s, err: %v\n", rule.Name, policy.ObjectMeta.Name, err)
			continue
		}

		if ok, err := mutation.IsRuleApplicableToResource(rawResource, rule.Resource); !ok {
			p.logger.Printf("Rule %d of policy %s is not applicable to the request", ruleIdx, policy.Name)
			return nil, err
		}

		err = p.applyRuleGenerators(rawResource, rule)
		if err != nil && patchingSets == mutation.PatchingSetsStopOnError {
			return nil, fmt.Errorf("Failed to apply generators from rule #%s: %v", rule.Name, err)
		}

		rulePatchesProcessed, err := mutation.ProcessPatches(rule.Patches, rawResource, patchingSets)
		if err != nil {
			return nil, fmt.Errorf("Failed to process patches from rule #%s: %v", rule.Name, err)
		}

		if rulePatchesProcessed != nil {
			policyPatches = append(policyPatches, rulePatchesProcessed...)
			p.logger.Printf("Rule %d: prepared %d patches", ruleIdx, len(rulePatchesProcessed))
			// TODO: add PolicyApplied events per rule for policy and resource
		} else {
			p.logger.Printf("Rule %d: no patches prepared", ruleIdx)
		}
	}

	// empty patch, return error to deny resource creation
	if policyPatches == nil {
		return nil, fmt.Errorf("no patches prepared")
	}

	return policyPatches, nil
}

// Applies "configMapGenerator" and "secretGenerator" described in PolicyRule
func (p *policyEngine) applyRuleGenerators(rawResource []byte, rule types.PolicyRule) error {
	kind := mutation.ParseKindFromObject(rawResource)

	// configMapGenerator and secretGenerator can be applied only to namespaces
	if kind == "Namespace" {
		namespaceName := mutation.ParseNameFromObject(rawResource)

		err := p.applyConfigGenerator(rule.ConfigMapGenerator, namespaceName, "ConfigMap")
		if err == nil {
			err = p.applyConfigGenerator(rule.SecretGenerator, namespaceName, "Secret")
		}
		return err
	}
	return nil
}

// Creates resourceKind (ConfigMap or Secret) with parameters specified in generator in cluster specified in request.
func (p *policyEngine) applyConfigGenerator(generator *types.PolicyConfigGenerator, namespace string, configKind string) error {
	if generator == nil {
		return nil
	}

	err := generator.Validate()
	if err != nil {
		return errors.New(fmt.Sprintf("Generator for '%s' is invalid: %s", configKind, err))
	}

	switch configKind {
	case "ConfigMap":
		err = p.kubeClient.GenerateConfigMap(*generator, namespace)
	case "Secret":
		err = p.kubeClient.GenerateSecret(*generator, namespace)
	default:
		err = errors.New(fmt.Sprintf("Unsupported config Kind '%s'", configKind))
	}

	if err != nil {
		return errors.New(fmt.Sprintf("Unable to apply generator for %s '%s/%s' : %s", configKind, namespace, generator.Name, err))
	}

	return nil
}
