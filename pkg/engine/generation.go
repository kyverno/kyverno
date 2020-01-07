package engine

import (
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/rbac"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

//GenerateNew returns the list of rules that are applicable on this policy and resource
func GenerateNew(policyContext PolicyContext) (resp response.EngineResponse) {
	policy := policyContext.Policy
	resource := policyContext.NewResource
	admissionInfo := policyContext.AdmissionInfo
	return filterRules(policy, resource, admissionInfo)
}

func filterRule(rule kyverno.Rule, resource unstructured.Unstructured, admissionInfo kyverno.RequestInfo) *response.RuleResponse {
	if !rule.HasGenerate() {
		return nil
	}
	if !rbac.MatchAdmissionInfo(rule, admissionInfo) {
		return nil
	}
	if !MatchesResourceDescription(resource, rule) {
		return nil
	}
	// build rule Response
	return &response.RuleResponse{
		Name: rule.Name,
		Type: "Generation",
	}
}

func filterRules(policy kyverno.ClusterPolicy, resource unstructured.Unstructured, admissionInfo kyverno.RequestInfo) response.EngineResponse {
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
		if ruleResp := filterRule(rule, resource, admissionInfo); ruleResp != nil {
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
		}
	}
	return resp
}
