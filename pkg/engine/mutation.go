package engine

import (
	"log"

	kubepolicy "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kube-policy/pkg/engine/mutation"
)

// Mutate performs mutation. Overlay first and then mutation patches
// TODO: pass in logger?
func Mutate(policy kubepolicy.Policy, rawResource []byte) []mutation.PatchBytes {
	var policyPatches []mutation.PatchBytes

	for i, rule := range policy.Spec.Rules {

		// Checks for preconditions
		// TODO: Rework PolicyEngine interface that it receives not a policy, but mutation object for
		// Mutate, validation for Validate and so on. It will allow to bring this checks outside of PolicyEngine
		// to common part as far as they present for all: mutation, validation, generation

		err := rule.Validate()
		if err != nil {
			log.Printf("Rule has invalid structure: rule number = %d, rule name = %s in policy %s, err: %v\n", i, rule.Name, policy.ObjectMeta.Name, err)
			continue
		}

		ok, err := mutation.IsRuleApplicableToResource(rawResource, rule.ResourceDescription)
		if err != nil {
			log.Printf("Rule has invalid data: rule number = %d, rule name = %s in policy %s, err: %v\n", i, rule.Name, policy.ObjectMeta.Name, err)
			continue
		}

		if !ok {
			log.Printf("Rule is not applicable t the request: rule number = %d, rule name = %s in policy %s, err: %v\n", i, rule.Name, policy.ObjectMeta.Name, err)
			continue
		}

		// Process Overlay

		if rule.Mutation.Overlay != nil {
			overlayPatches, err := mutation.ProcessOverlay(rule.Mutation.Overlay, rawResource)
			if err != nil {
				log.Printf("Overlay application failed: rule number = %d, rule name = %s in policy %s, err: %v\n", i, rule.Name, policy.ObjectMeta.Name, err)
			} else {
				policyPatches = append(policyPatches, overlayPatches...)
			}
		}

		// Process Patches

		if rule.Mutation.Patches != nil {
			processedPatches, err := mutation.ProcessPatches(rule.Mutation.Patches, rawResource)
			if err != nil {
				log.Printf("Patches application failed: rule number = %d, rule name = %s in policy %s, err: %v\n", i, rule.Name, policy.ObjectMeta.Name, err)
			} else {
				policyPatches = append(policyPatches, processedPatches...)
			}
		}

	}

	return policyPatches
}
