package engine

import (
	"github.com/golang/glog"
	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Mutate performs mutation. Overlay first and then mutation patches
// TODO: return events and violations
func Mutate(policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind) ([]PatchBytes, []byte) {
	var policyPatches []PatchBytes
	var processedPatches []PatchBytes
	var err error
	patchedDocument := rawResource

	for _, rule := range policy.Spec.Rules {
		if rule.Mutation == nil {
			continue
		}

		ok := ResourceMeetsDescription(rawResource, rule.ResourceDescription, gvk)
		if !ok {
			glog.Infof("Rule \"%s\" is not applicable to resource\n", rule.Name)
			continue
		}

		// Process Overlay

		if rule.Mutation.Overlay != nil {
			overlayPatches, err := ProcessOverlay(policy, rawResource, gvk)
			if err != nil {
				glog.Warningf("Overlay application has failed for rule %s in policy %s, err: %v\n", rule.Name, policy.ObjectMeta.Name, err)
			} else {
				policyPatches = append(policyPatches, overlayPatches...)
			}
		}

		// Process Patches

		if rule.Mutation.Patches != nil {
			processedPatches, patchedDocument, err = ProcessPatches(rule.Mutation.Patches, patchedDocument)
			if err != nil {
				glog.Warningf("Patches application has failed for rule %s in policy %s, err: %v\n", rule.Name, policy.ObjectMeta.Name, err)
			} else {
				policyPatches = append(policyPatches, processedPatches...)
			}
		}
	}

	return policyPatches, patchedDocument
}
