package engine

import (
	"time"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/engine/generate"
	"github.com/nirmata/kyverno/pkg/engine/response"
)

//Generate apply generation rules on a resource
func Generate(policyContext PolicyContext) (resp response.EngineResponse) {
	policy := policyContext.Policy
	ns := policyContext.NewResource
	client := policyContext.Client
	ctx := policyContext.Context

	startTime := time.Now()
	// policy information
	func() {
		// set policy information
		resp.PolicyResponse.Policy = policy.Name
		// resource details
		resp.PolicyResponse.Resource.Name = ns.GetName()
		resp.PolicyResponse.Resource.Kind = ns.GetKind()
		resp.PolicyResponse.Resource.APIVersion = ns.GetAPIVersion()
	}()
	glog.V(4).Infof("started applying generation rules of policy %q (%v)", policy.Name, startTime)
	defer func() {
		resp.PolicyResponse.ProcessingTime = time.Since(startTime)
		glog.V(4).Infof("finished applying generation rules policy %v (%v)", policy.Name, resp.PolicyResponse.ProcessingTime)
		glog.V(4).Infof("Generation Rules appplied succesfully count %v for policy %q", resp.PolicyResponse.RulesAppliedCount, policy.Name)
	}()
	incrementAppliedRuleCount := func() {
		// rules applied succesfully count
		resp.PolicyResponse.RulesAppliedCount++
	}
	for _, rule := range policy.Spec.Rules {
		if !rule.HasGenerate() {
			continue
		}
		glog.V(4).Infof("applying policy %s generate rule %s on resource %s/%s/%s", policy.Name, rule.Name, ns.GetKind(), ns.GetNamespace(), ns.GetName())
		ruleResponse := generate.ApplyRuleGenerator(ctx, client, ns, rule, policy.GetCreationTimestamp())
		resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResponse)
		incrementAppliedRuleCount()
	}
	// set resource in reponse
	resp.PatchedResource = ns
	return resp
}
