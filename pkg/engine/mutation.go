package engine

import (
	"github.com/golang/glog"
	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/info"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Mutate performs mutation. Overlay first and then mutation patches
func Mutate(policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind) ([][]byte, []*info.RuleInfo) {
	//

	var allPatches [][]byte
	patchedDocument := rawResource
	ris := []*info.RuleInfo{}

	for _, rule := range policy.Spec.Rules {
		if rule.Mutation == nil {
			continue
		}

		// check if the resource satisfies the filter conditions defined in the rule
		//TODO: this needs to be extracted, to filter the resource so that we can avoid passing resources that
		// dont statisfy a policy rule resource description
		ok := ResourceMeetsDescription(rawResource, rule.MatchResources.ResourceDescription, rule.ExcludeResources.ResourceDescription, gvk)
		if !ok {
			name := ParseNameFromObject(rawResource)
			namespace := ParseNamespaceFromObject(rawResource)
			glog.V(3).Infof("resource %s/%s does not satisfy the resource description for the rule ", namespace, name)
			continue
		}
		ri := info.NewRuleInfo(rule.Name, info.Mutation)

		// Process Overlay
		if rule.Mutation.Overlay != nil {
			overlayPatches, err := processOverlay(rule, rawResource, gvk)
			if err == nil {
				if len(overlayPatches) == 0 {
					// if array elements dont match then we skip(nil patch, no error)
					// or if acnohor is defined and doenst match
					// policy is not applicable
					continue
				}
				glog.V(4).Infof("overlay applied succesfully on resource")
				ri.Add("Overlay succesfully applied")
				patch := JoinPatches(overlayPatches)
				allPatches = append(allPatches, overlayPatches...)
				// update rule information
				// strip slashes from string
				ri.Changes = string(patch)
			} else {
				glog.V(4).Infof("failed to apply overlay: %v", err)
				ri.Fail()
				ri.Addf("failed to apply overlay: %v", err)
			}
		}

		// Process Patches
		if len(rule.Mutation.Patches) != 0 {
			rulePatches, errs := processPatches(rule, patchedDocument)
			if len(errs) > 0 {
				ri.Fail()
				for _, err := range errs {
					glog.V(4).Infof("failed to apply patches: %v", err)
					ri.Addf("patches application has failed, err %v.", err)
				}
			} else {
				glog.V(4).Infof("patches applied succesfully on resource")
				ri.Addf("Patches succesfully applied.")
				allPatches = append(allPatches, rulePatches...)
			}
		}
		ris = append(ris, ri)
	}

	return allPatches, ris
}
