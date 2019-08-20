package engine

import (
	"reflect"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	"github.com/nirmata/kyverno/pkg/info"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Mutate performs mutation. Overlay first and then mutation patches

func Mutate(policy kyverno.Policy, resource unstructured.Unstructured) (response EngineResponse) {
	// var response EngineResponse
	var allPatches, rulePatches [][]byte
	var err error
	var errs []error
	ris := []info.RuleInfo{}
	startTime := time.Now()
	glog.V(4).Infof("started applying mutation rules of policy %q (%v)", policy.Name, startTime)
	defer func() {
		response.ExecutionTime = time.Since(startTime)
		glog.V(4).Infof("finished applying mutation rules policy %v (%v)", policy.Name, response.ExecutionTime)
		glog.V(4).Infof("Mutation Rules appplied succesfully count %v for policy %q", response.RulesAppliedCount, policy.Name)
	}()
	incrementAppliedRuleCount := func() {
		// rules applied succesfully count
		response.RulesAppliedCount++
	}

	patchedDocument, err := resource.MarshalJSON()
	if err != nil {
		glog.Errorf("unable to marshal resource : %v\n", err)
	}

	if err != nil {
		glog.V(4).Infof("unable to marshal resource : %v", err)
		response.PatchedResource = resource
		return response
	}

	for _, rule := range policy.Spec.Rules {
		if reflect.DeepEqual(rule.Mutation, kyverno.Mutation{}) {
			continue
		}

		// check if the resource satisfies the filter conditions defined in the rule
		//TODO: this needs to be extracted, to filter the resource so that we can avoid passing resources that
		// dont statisfy a policy rule resource description
		ok := MatchesResourceDescription(resource, rule)
		if !ok {
			glog.V(4).Infof("resource %s/%s does not satisfy the resource description for the rule ", resource.GetNamespace(), resource.GetName())
			continue
		}

		ruleInfo := info.NewRuleInfo(rule.Name, info.Mutation)

		// Process Overlay
		if rule.Mutation.Overlay != nil {
			rulePatches, err = processOverlay(rule, patchedDocument)
			if err == nil {
				if len(rulePatches) == 0 {
					// if array elements dont match then we skip(nil patch, no error)
					// or if acnohor is defined and doenst match
					// policy is not applicable
					glog.V(4).Info("overlay does not match, so skipping applying rule")
					continue
				}

				ruleInfo.Addf("Rule %s: Overlay succesfully applied.", rule.Name)

				// strip slashes from string
				ruleInfo.Patches = rulePatches
				allPatches = append(allPatches, rulePatches...)

				glog.V(4).Infof("overlay applied succesfully on resource %s/%s", resource.GetNamespace(), resource.GetName())
			} else {
				glog.V(4).Infof("failed to apply overlay: %v", err)
				ruleInfo.Fail()
				ruleInfo.Addf("failed to apply overlay: %v", err)
			}
			incrementAppliedRuleCount()
		}

		// Process Patches
		if len(rule.Mutation.Patches) != 0 {
			rulePatches, errs = processPatches(rule, patchedDocument)
			if len(errs) > 0 {
				ruleInfo.Fail()
				for _, err := range errs {
					glog.V(4).Infof("failed to apply patches: %v", err)
					ruleInfo.Addf("patches application has failed, err %v.", err)
				}
			} else {
				glog.V(4).Infof("patches applied succesfully on resource %s/%s", resource.GetNamespace(), resource.GetName())
				ruleInfo.Addf("Patches succesfully applied.")

				ruleInfo.Patches = rulePatches
				allPatches = append(allPatches, rulePatches...)
			}
			incrementAppliedRuleCount()
		}

		patchedDocument, err = ApplyPatches(patchedDocument, rulePatches)
		if err != nil {
			glog.Errorf("Failed to apply patches on ruleName=%s, err%v\n:", rule.Name, err)
		}

		ris = append(ris, ruleInfo)
	}

	patchedResource, err := ConvertToUnstructured(patchedDocument)
	if err != nil {
		glog.Errorf("Failed to convert patched resource to unstructuredtype, err%v\n:", err)
		response.PatchedResource = resource
		return response
	}

	response.Patches = allPatches
	response.PatchedResource = *patchedResource
	response.RuleInfos = ris
	return response
}
