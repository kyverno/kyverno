package policyengine

import (
	kubepolicy "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kube-policy/pkg/policyengine/mutation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Mutate performs mutation. Overlay first and then mutation patches
func (p *policyEngine) Mutate(policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind) []mutation.PatchBytes {
	var policyPatches []mutation.PatchBytes

	for i, rule := range policy.Spec.Rules {

		// Checks for preconditions
		// TODO: Rework PolicyEngine interface that it receives not a policy, but mutation object for
		// Mutate, validation for Validate and so on. It will allow to bring this checks outside of PolicyEngine
		// to common part as far as they present for all: mutation, validation, generation

		err := rule.Validate()
		if err != nil {
			p.logger.Printf("Rule has invalid structure: rule number = %d, rule name = %s in policy %s, err: %v\n", i, rule.Name, policy.ObjectMeta.Name, err)
			continue
		}

		ok, err := mutation.ResourceMeetsRules(rawResource, rule.ResourceDescription, gvk)
		if err != nil {
			p.logger.Printf("Rule has invalid data: rule number = %d, rule name = %s in policy %s, err: %v\n", i, rule.Name, policy.ObjectMeta.Name, err)
			continue
		}

		if !ok {
			p.logger.Printf("Rule is not applicable t the request: rule number = %d, rule name = %s in policy %s, err: %v\n", i, rule.Name, policy.ObjectMeta.Name, err)
			continue
		}

		if rule.Mutation == nil {
			continue
		}

		// Process Overlay

		if rule.Mutation.Overlay != nil {
			overlayPatches, err := mutation.ProcessOverlay(rule.Mutation.Overlay, rawResource)
			if err != nil {
				p.logger.Printf("Overlay application failed: rule number = %d, rule name = %s in policy %s, err: %v\n", i, rule.Name, policy.ObjectMeta.Name, err)
			} else {
				policyPatches = append(policyPatches, overlayPatches...)
			}
		}

		// Process Patches

		if rule.Mutation.Patches != nil {
			processedPatches, err := mutation.ProcessPatches(rule.Mutation.Patches, rawResource)
			if err != nil {
				p.logger.Printf("Patches application failed: rule number = %d, rule name = %s in policy %s, err: %v\n", i, rule.Name, policy.ObjectMeta.Name, err)
			} else {
				policyPatches = append(policyPatches, processedPatches...)
			}
		}
	}

	return policyPatches
}
