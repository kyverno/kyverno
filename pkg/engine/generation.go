package engine

import (
	"fmt"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/engine/utils"
	"github.com/nirmata/kyverno/pkg/engine/variables"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Generate checks for validity of generate rule on the resource
// 1. validate variables to be susbtitute in the general ruleInfo (match,exclude,condition)
//    - the caller has to check the ruleResponse to determine whether the path exist
// 2. returns the list of rules that are applicable on this policy and resource, if 1 succeed
func Generate(policyContext PolicyContext) (resp response.EngineResponse) {
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
	if !MatchesResourceDescription(resource, rule, admissionInfo) {
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
		if paths := validateGeneralRuleInfoVariables(ctx, rule); len(paths) != 0 {
			glog.Infof("referenced path not present in generate rule %s, resource %s/%s/%s, path: %s", rule.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName(), paths)
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules,
				newPathNotPresentRuleResponse(rule.Name, utils.Mutation.String(), fmt.Sprintf("path not present: %s", paths)))
			continue
		}

		if ruleResp := filterRule(rule, resource, admissionInfo, ctx); ruleResp != nil {
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
		}
	}
	return resp
}
