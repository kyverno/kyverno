package engine

import (
	"strings"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Mutate performs mutation. Overlay first and then mutation patches
func Mutate(policy kyverno.ClusterPolicy, resource unstructured.Unstructured) (response EngineResponse) {
	startTime := time.Now()
	// policy information
	func() {
		// set policy information
		response.PolicyResponse.Policy = policy.Name
		// resource details
		response.PolicyResponse.Resource.Name = resource.GetName()
		response.PolicyResponse.Resource.Namespace = resource.GetNamespace()
		response.PolicyResponse.Resource.Kind = resource.GetKind()
		response.PolicyResponse.Resource.APIVersion = resource.GetAPIVersion()
	}()
	glog.V(4).Infof("started applying mutation rules of policy %q (%v)", policy.Name, startTime)
	defer func() {
		response.PolicyResponse.ProcessingTime = time.Since(startTime)
		glog.V(4).Infof("finished applying mutation rules policy %v (%v)", policy.Name, response.PolicyResponse.ProcessingTime)
		glog.V(4).Infof("Mutation Rules appplied succesfully count %v for policy %q", response.PolicyResponse.RulesAppliedCount, policy.Name)
	}()
	incrementAppliedRuleCount := func() {
		// rules applied succesfully count
		response.PolicyResponse.RulesAppliedCount++
	}

	var patchedResource unstructured.Unstructured

	for _, rule := range policy.Spec.Rules {
		//TODO: to be checked before calling the resources as well
		if !rule.HasMutate() {
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
		// Process Overlay
		if rule.Mutation.Overlay != nil {
			var ruleResponse RuleResponse
			ruleResponse, patchedResource = processOverlay(rule, resource)
			if strings.Contains(ruleResponse.Message, "policy not applied") {
				// overlay pattern does not match the resource conditions
				continue
			}
			response.PolicyResponse.Rules = append(response.PolicyResponse.Rules, ruleResponse)
			incrementAppliedRuleCount()
		}

		// Process Patches
		if rule.Mutation.Patches != nil {
			var ruleResponse RuleResponse
			ruleResponse, patchedResource = processPatches(rule, resource)
			response.PolicyResponse.Rules = append(response.PolicyResponse.Rules, ruleResponse)
			incrementAppliedRuleCount()
		}
	}
	// send the patched resource
	response.PatchedResource = patchedResource
	return response
}
