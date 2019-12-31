package engine

import (
	"time"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/engine/response"
)

// Mutate performs mutation. Overlay first and then mutation patches
func Mutate(policyContext PolicyContext) (resp response.EngineResponse) {
	startTime := time.Now()
	policy := policyContext.Policy
	resource := policyContext.NewResource
	ctx := policyContext.Context

	// policy information
	func() {
		// set policy information
		resp.PolicyResponse.Policy = policy.Name
		// resource details
		resp.PolicyResponse.Resource.Name = resource.GetName()
		resp.PolicyResponse.Resource.Namespace = resource.GetNamespace()
		resp.PolicyResponse.Resource.Kind = resource.GetKind()
		resp.PolicyResponse.Resource.APIVersion = resource.GetAPIVersion()
	}()
	glog.V(4).Infof("started applying mutation rules of policy %q (%v)", policy.Name, startTime)
	defer func() {
		resp.PolicyResponse.ProcessingTime = time.Since(startTime)
		glog.V(4).Infof("finished applying mutation rules policy %v (%v)", policy.Name, resp.PolicyResponse.ProcessingTime)
		glog.V(4).Infof("Mutation Rules appplied count %v for policy %q", resp.PolicyResponse.RulesAppliedCount, policy.Name)
	}()
	incrementAppliedRuleCount := func() {
		// rules applied succesfully count
		resp.PolicyResponse.RulesAppliedCount++
	}

	patchedResource := policyContext.NewResource

	for _, rule := range policy.Spec.Rules {
		//TODO: to be checked before calling the resources as well
		if !rule.HasMutate() {
			continue
		}

		startTime := time.Now()
		if !matchAdmissionInfo(rule, policyContext.AdmissionInfo) {
			glog.V(3).Infof("rule '%s' cannot be applied on %s/%s/%s, admission permission: %v",
				rule.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName(), policyContext.AdmissionInfo)
			continue
		}
		glog.V(4).Infof("Time: Mutate matchAdmissionInfo %v", time.Since(startTime))

		// check if the resource satisfies the filter conditions defined in the rule
		//TODO: this needs to be extracted, to filter the resource so that we can avoid passing resources that
		// dont statisfy a policy rule resource description
		ok := MatchesResourceDescription(resource, rule)
		if !ok {
			glog.V(4).Infof("resource %s/%s does not satisfy the resource description for the rule ", resource.GetNamespace(), resource.GetName())
			continue
		}
		// Process Overlay
		if rule.Mutation.Overlay != nil {
			var ruleResponse response.RuleResponse
			ruleResponse, patchedResource = processOverlay(ctx, rule, patchedResource)
			if ruleResponse.Success == true && ruleResponse.Patches == nil {
				// overlay pattern does not match the resource conditions
				glog.V(4).Infof(ruleResponse.Message)
				continue
			} else if ruleResponse.Success == true {
				glog.Infof("Mutate overlay in rule '%s' successfully applied on %s/%s/%s", rule.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName())
			}

			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResponse)
			incrementAppliedRuleCount()
		}

		// Process Patches
		if rule.Mutation.Patches != nil {
			var ruleResponse response.RuleResponse
			ruleResponse, patchedResource = processPatches(rule, patchedResource)
			glog.Infof("Mutate patches in rule '%s' successfully applied on %s/%s/%s", rule.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName())
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResponse)
			incrementAppliedRuleCount()
		}
	}
	// send the patched resource
	resp.PatchedResource = patchedResource
	return resp
}
