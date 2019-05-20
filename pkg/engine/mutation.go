package engine

import (
	"log"

	kubepolicy "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kube-policy/pkg/engine/mutation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Mutate performs mutation. Overlay first and then mutation patches
// TODO: return events and violations
func Mutate(policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind) []mutation.PatchBytes {
	var policyPatches []mutation.PatchBytes

	for _, rule := range policy.Spec.Rules {
		if rule.Mutation == nil {
			continue
		}

		ok := ResourceMeetsDescription(rawResource, rule.ResourceDescription, gvk)
		if !ok {
			log.Printf("Rule \"%s\" is not applicable to resource\n", rule.Name)
			continue
		}

		// Process Overlay

		if rule.Mutation.Overlay != nil {
			overlayPatches, err := mutation.ProcessOverlay(rule.Mutation.Overlay, rawResource)
			if err != nil {
				log.Printf("Overlay application has failed for rule %s in policy %s, err: %v\n", rule.Name, policy.ObjectMeta.Name, err)
			} else {
				policyPatches = append(policyPatches, overlayPatches...)
			}
		}

		// Process Patches

		if rule.Mutation.Patches != nil {
			processedPatches, err := mutation.ProcessPatches(rule.Mutation.Patches, rawResource)
			if err != nil {
				log.Printf("Patches application has failed for rule %s in policy %s, err: %v\n", rule.Name, policy.ObjectMeta.Name, err)
			} else {
				policyPatches = append(policyPatches, processedPatches...)
			}
		}
	}

	return policyPatches
}
