package engine

import (
	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/result"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Mutate performs mutation. Overlay first and then mutation patches
func Mutate(policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind) ([]PatchBytes, result.Result) {
	var allPatches []PatchBytes

	patchedDocument := rawResource
	policyResult := result.NewPolicyApplicationResult(policy.Name)

	for _, rule := range policy.Spec.Rules {
		if rule.Mutation == nil {
			continue
		}

		ruleApplicationResult := result.NewRuleApplicationResult(rule.Name)

		ok := ResourceMeetsDescription(rawResource, rule.ResourceDescription, gvk)
		if !ok {
			ruleApplicationResult.AddMessagef("Rule %s is not applicable to resource\n", rule.Name)
			policyResult = result.Append(policyResult, &ruleApplicationResult)
			continue
		}

		// Process Overlay

		if rule.Mutation.Overlay != nil {
			overlayPatches, ruleResult := ProcessOverlay(rule.Mutation.Overlay, rawResource, gvk)
			if result.Success != ruleResult.GetReason() {
				ruleApplicationResult.MergeWith(&ruleResult)
				ruleApplicationResult.AddMessagef("Overlay application has failed for rule %s in policy %s\n", rule.Name, policy.ObjectMeta.Name)
			} else {
				ruleApplicationResult.AddMessagef("Success")
				allPatches = append(allPatches, overlayPatches...)
			}
		}

		// Process Patches

		if rule.Mutation.Patches != nil {
			rulePatches, ruleResult := ProcessPatches(rule.Mutation.Patches, patchedDocument)

			if result.Success != ruleResult.GetReason() {
				ruleApplicationResult.MergeWith(&ruleResult)
				ruleApplicationResult.AddMessagef("Patches application has failed for rule %s in policy %s\n", rule.Name, policy.ObjectMeta.Name)
			} else {
				ruleApplicationResult.AddMessagef("Success")
				allPatches = append(allPatches, rulePatches...)
			}
		}
	}

	return allPatches, policyResult
}
