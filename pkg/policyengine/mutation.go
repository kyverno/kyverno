package policyengine

import (
	"fmt"
	"log"

	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kube-policy/pkg/policyengine/mutation"
)

// Mutation should be claed to process the mutation rules on the resource
//TODO return []event.Info
func Mutation(logger *log.Logger, policy types.Policy, rawResource []byte) ([]mutation.PatchBytes, error) {
	patchingSets := mutation.GetPolicyPatchingSets(policy)
	var policyPatches []mutation.PatchBytes

	for ruleIdx, rule := range policy.Spec.Rules {
		err := rule.Validate()
		if err != nil {
			logger.Printf("Invalid rule detected: #%s in policy %s, err: %v\n", rule.Name, policy.ObjectMeta.Name, err)
			continue
		}

		if ok, err := mutation.IsRuleApplicableToResource(rawResource, rule.Resource); !ok {
			logger.Printf("Rule %d of policy %s is not applicable to the request", ruleIdx, policy.Name)
			return nil, err
		}

		if err != nil && patchingSets == mutation.PatchingSetsStopOnError {
			return nil, fmt.Errorf("Failed to apply generators from rule #%s: %v", rule.Name, err)
		}

		rulePatchesProcessed, err := mutation.ProcessPatches(rule.Patches, rawResource, patchingSets)
		if err != nil {
			return nil, fmt.Errorf("Failed to process patches from rule #%s: %v", rule.Name, err)
		}

		if rulePatchesProcessed != nil {
			policyPatches = append(policyPatches, rulePatchesProcessed...)
			logger.Printf("Rule %d: prepared %d patches", ruleIdx, len(rulePatchesProcessed))
			// TODO: add PolicyApplied events per rule for policy and resource
		} else {
			logger.Printf("Rule %d: no patches prepared", ruleIdx)
		}
	}

	// empty patch, return error to deny resource creation
	if policyPatches == nil {
		return nil, fmt.Errorf("no patches prepared")
	}

	return policyPatches, nil
}
