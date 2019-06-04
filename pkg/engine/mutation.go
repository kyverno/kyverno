package engine

import (
	"github.com/golang/glog"
	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/result"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Mutate performs mutation. Overlay first and then mutation patches
func Mutate(policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind) ([]PatchBytes, result.Result) {
	var allPatches []PatchBytes
	policyResult := result.NewPolicyApplicationResult(policy.Name)
	var err error
	patchedDocument := rawResource

	for _, rule := range policy.Spec.Rules {
		if rule.Mutation == nil {
			continue
		}

		ok := ResourceMeetsDescription(rawResource, rule.ResourceDescription, gvk)
		if !ok {
			// TODO add to res
			glog.Infof("Rule \"%s\" is not applicable to resource\n", rule.Name)
			continue
		}

		// Process Overlay

		if rule.Mutation.Overlay != nil {
			overlayPatches, err := ProcessOverlay(policy, rawResource, gvk)
			if err != nil {
				// TODO add to res
				glog.Warningf("Overlay application has failed for rule %s in policy %s, err: %v\n", rule.Name, policy.ObjectMeta.Name, err)
			} else {
				allPatches = append(allPatches, overlayPatches...)
			}
		}

		// Process Patches

		if rule.Mutation.Patches != nil {
			rulePatches, _ /*ruleResult*/ := ProcessPatches(rule.Mutation.Patches, patchedDocument)
			if err != nil {
				// TODO add to res
				glog.Warningf("Patches application has failed for rule %s in policy %s, err: %v\n", rule.Name, policy.ObjectMeta.Name, err)
			} else {
				allPatches = append(allPatches, rulePatches...)
			}
		}
	}

	return allPatches, policyResult
}
