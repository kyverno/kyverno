package engine

import (
	"time"

	"github.com/go-logr/logr"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/engine/variables"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

	logger := log.Log.WithName("Generate").WithValues("policy", policy.Name, "kind", resource.GetKind(), "namespace", resource.GetNamespace(), "name", resource.GetName())

	return filterRules(policy, resource, admissionInfo, ctx, logger, policyContext.ExcludeGroupRole)
}

func filterRule(rule kyverno.Rule, resource unstructured.Unstructured, admissionInfo kyverno.RequestInfo, ctx context.EvalInterface, log logr.Logger, excludeGroupRole []string) *response.RuleResponse {
	if !rule.HasGenerate() {
		return nil
	}

	startTime := time.Now()
	if err := MatchesResourceDescription(resource, rule, admissionInfo, excludeGroupRole); err != nil {
		return nil
	}
	// operate on the copy of the conditions, as we perform variable substitution
	copyConditions := copyConditions(rule.Conditions)

	// evaluate pre-conditions
	if !variables.EvaluateConditions(log, ctx, copyConditions) {
		log.V(4).Info("preconditions not satisfied, skipping rule", "rule", rule.Name)
		return nil
	}
	// build rule Response
	return &response.RuleResponse{
		Name:    rule.Name,
		Type:    "Generation",
		Success: true,
		RuleStats: response.RuleStats{
			ProcessingTime: time.Since(startTime),
		},
	}
}

func filterRules(policy kyverno.ClusterPolicy, resource unstructured.Unstructured, admissionInfo kyverno.RequestInfo, ctx context.EvalInterface, log logr.Logger, excludeGroupRole []string) response.EngineResponse {
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
		if ruleResp := filterRule(rule, resource, admissionInfo, ctx, log, excludeGroupRole); ruleResp != nil {
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
		}
	}
	return resp
}
