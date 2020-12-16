package engine

import (
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

// Generate checks for validity of generate rule on the resource
// 1. validate variables to be substitute in the general ruleInfo (match,exclude,condition)
//    - the caller has to check the ruleResponse to determine whether the path exist
// 2. returns the list of rules that are applicable on this policy and resource, if 1 succeed
func Generate(policyContext PolicyContext) (resp response.EngineResponse) {
	return filterRules(policyContext)
}

func filterRules(policyContext PolicyContext) response.EngineResponse {
	kind := policyContext.NewResource.GetKind()
	name := policyContext.NewResource.GetName()
	namespace := policyContext.NewResource.GetNamespace()

	resp := response.EngineResponse{
		PolicyResponse: response.PolicyResponse{
			Policy: policyContext.Policy.Name,
			Resource: response.ResourceSpec{
				Kind:      kind,
				Name:      name,
				Namespace: namespace,
			},
		},
	}

	if policyContext.ExcludeResourceFunc(kind, namespace, name) {
		log.Log.WithName("Generate").Info("resource excluded", "kind", kind, "namespace", namespace, "name", name)
		return resp
	}

	for _, rule := range policyContext.Policy.Spec.Rules {
		if ruleResp := filterRule(rule, policyContext); ruleResp != nil {
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
		}
	}

	return resp
}

func filterRule(rule kyverno.Rule, policyContext PolicyContext) *response.RuleResponse {
	if !rule.HasGenerate() {
		return nil
	}

	startTime := time.Now()

	policy := policyContext.Policy
	newResource := policyContext.NewResource
	oldResource := policyContext.OldResource
	admissionInfo := policyContext.AdmissionInfo
	ctx := policyContext.Context
	resCache := policyContext.ResourceCache
	jsonContext := policyContext.JSONContext
	excludeGroupRole := policyContext.ExcludeGroupRole

	logger := log.Log.WithName("Generate").WithValues("policy", policy.Name,
		"kind", newResource.GetKind(), "namespace", newResource.GetNamespace(), "name", newResource.GetName())

	if err := MatchesResourceDescription(newResource, rule, admissionInfo, excludeGroupRole); err != nil {

		// if the oldResource matched, return "false" to delete GR for it
		if err := MatchesResourceDescription(oldResource, rule, admissionInfo, excludeGroupRole); err == nil {
			return &response.RuleResponse{
				Name:    rule.Name,
				Type:    "Generation",
				Success: false,
				RuleStats: response.RuleStats{
					ProcessingTime: time.Since(startTime),
				},
			}
		}

		return nil
	}

	// add configmap json data to context
	if err := AddResourceToContext(logger, rule.Context, resCache, jsonContext); err != nil {
		logger.V(4).Info("cannot add configmaps to context", "reason", err.Error())
		return nil
	}

	// operate on the copy of the conditions, as we perform variable substitution
	copyConditions := copyConditions(rule.Conditions)

	// evaluate pre-conditions
	if !variables.EvaluateConditions(logger, ctx, copyConditions) {
		logger.V(4).Info("preconditions not satisfied, skipping rule", "rule", rule.Name)
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
