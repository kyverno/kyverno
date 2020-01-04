package engine

import (
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/rbac"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/engine/variables"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

//GenerateNew returns the list of rules that are applicable on this policy and resource
func GenerateNew(policyContext PolicyContext) (resp response.EngineResponse) {
	policy := policyContext.Policy
	resource := policyContext.NewResource
	admissionInfo := policyContext.AdmissionInfo
	ctx := policyContext.Context
	return filterRules(policy, resource, admissionInfo, ctx)
}

func filterRule(rule kyverno.Rule, resource unstructured.Unstructured, admissionInfo kyverno.RequestInfo, ctx context.EvalInterface) *response.RuleResponse {
	if !rule.HasGenerate() {
		return nil
	}
	if !rbac.MatchAdmissionInfo(rule, admissionInfo) {
		return nil
	}
	if !MatchesResourceDescription(resource, rule) {
		return nil
	}

	// evaluate pre-conditions
	if !variables.EvaluateConditions(ctx, rule.Conditions) {
		glog.V(4).Infof("resource %s/%s does not satisfy the conditions for the rule ", resource.GetNamespace(), resource.GetName())
		return nil
	}
	// build rule Response
	return &response.RuleResponse{
		Name: rule.Name,
		Type: "Generation",
	}
}

func filterRules(policy kyverno.ClusterPolicy, resource unstructured.Unstructured, admissionInfo kyverno.RequestInfo, ctx context.EvalInterface) response.EngineResponse {
	resp := response.EngineResponse{
		PolicyResponse: response.PolicyResponse{
			Policy: policy.Name,
			Resource: response.ResourceSpec{
				Kind:      resource.GetKind(),
				Name:      resource.GetName(),
				Namespace: resource.GetNamespace(),
			},
		},
	}

	for _, rule := range policy.Spec.Rules {
		if ruleResp := filterRule(rule, resource, admissionInfo, ctx); ruleResp != nil {
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
		}
	}
	return resp
}

// //Generate apply generation rules on a resource
// func Generate(policyContext PolicyContext) (resp response.EngineResponse) {
// 	policy := policyContext.Policy
// 	ns := policyContext.NewResource
// 	client := policyContext.Client
// 	ctx := policyContext.Context

// 	startTime := time.Now()
// 	// policy information
// 	func() {
// 		// set policy information
// 		resp.PolicyResponse.Policy = policy.Name
// 		// resource details
// 		resp.PolicyResponse.Resource.Name = ns.GetName()
// 		resp.PolicyResponse.Resource.Kind = ns.GetKind()
// 		resp.PolicyResponse.Resource.APIVersion = ns.GetAPIVersion()
// 	}()
// 	glog.V(4).Infof("started applying generation rules of policy %q (%v)", policy.Name, startTime)
// 	defer func() {
// 		resp.PolicyResponse.ProcessingTime = time.Since(startTime)
// 		glog.V(4).Infof("finished applying generation rules policy %v (%v)", policy.Name, resp.PolicyResponse.ProcessingTime)
// 		glog.V(4).Infof("Generation Rules appplied succesfully count %v for policy %q", resp.PolicyResponse.RulesAppliedCount, policy.Name)
// 	}()
// 	incrementAppliedRuleCount := func() {
// 		// rules applied succesfully count
// 		resp.PolicyResponse.RulesAppliedCount++
// 	}
// 	for _, rule := range policy.Spec.Rules {
// 		if !rule.HasGenerate() {
// 			continue
// 		}
// 		glog.V(4).Infof("applying policy %s generate rule %s on resource %s/%s/%s", policy.Name, rule.Name, ns.GetKind(), ns.GetNamespace(), ns.GetName())
// 		ruleResponse := generate.ApplyRuleGenerator(ctx, client, ns, rule, policy.GetCreationTimestamp())
// 		resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResponse)
// 		incrementAppliedRuleCount()
// 	}
// 	// set resource in reponse
// 	resp.PatchedResource = ns
// 	return resp
// }
