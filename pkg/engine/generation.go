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
	if !rbac.MatchAdmissionInfo(rule, admissionInfo) {
		return nil
	}
	if !MatchesResourceDescription(resource, rule) {
		return nil
	}
	// operate on the copy of the conditions, as we perform variable substitution
	copyConditions := copyConditions(rule.Conditions)

	// evaluate pre-conditions
	if !variables.EvaluateConditions(ctx, copyConditions) {
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
