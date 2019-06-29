package engine

import (
	"github.com/golang/glog"
	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/info"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Mutate performs mutation. Overlay first and then mutation patches
func Mutate(policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind) ([]PatchBytes, []*info.RuleInfo) {
	var allPatches []PatchBytes
	patchedDocument := rawResource
	ris := []*info.RuleInfo{}

	for _, rule := range policy.Spec.Rules {
		if rule.Mutation == nil {
			continue
		}
		ri := info.NewRuleInfo(rule.Name, info.Mutation)

		ok := ResourceMeetsDescription(rawResource, rule.ResourceDescription, gvk)
		if !ok {
			glog.V(3).Infof("Not applicable on specified resource kind%s", gvk.Kind)
			continue
		}
		// Process Overlay
		if rule.Mutation.Overlay != nil {
			overlayPatches, err := ProcessOverlay(rule, rawResource, gvk)
			if err != nil {
				ri.Fail()
				ri.Addf("Rule %s: Overlay application has failed, err %s.", rule.Name, err)
			} else {
				ri.Addf("Rule %s: Overlay succesfully applied.", rule.Name)
				//TODO: patchbytes -> string
				//glog.V(3).Info(" Overlay succesfully applied. Patch %s", string(overlayPatches))
				allPatches = append(allPatches, overlayPatches...)
			}
		}

		// Process Patches
		if len(rule.Mutation.Patches) != 0 {
			rulePatches, errs := ProcessPatches(rule, patchedDocument)
			if len(errs) > 0 {
				ri.Fail()
				for _, err := range errs {
					ri.Addf("Rule %s: Patches application has failed, err %s.", rule.Name, err)
				}
			} else {
				ri.Addf("Rule %s: Patches succesfully applied.", rule.Name)
				//TODO: patchbytes -> string
				//glog.V(3).Info("Patches succesfully applied. Patch %s", string(overlayPatches))
				allPatches = append(allPatches, rulePatches...)
			}
		}
		ris = append(ris, ri)
	}

	return allPatches, ris
}
