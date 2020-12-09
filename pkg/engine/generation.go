package engine

import (
	"time"

	"github.com/go-logr/logr"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/resourcecache"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Generate checks for validity of generate rule on the resource
// 1. validate variables to be susbtitute in the general ruleInfo (match,exclude,condition)
//    - the caller has to check the ruleResponse to determine whether the path exist
// 2. returns the list of rules that are applicable on this policy and resource, if 1 succeed
func Generate(policyContext PolicyContext) (resp response.EngineResponse) {
	policy := policyContext.Policy
	new := policyContext.NewResource
	old := policyContext.OldResource
	admissionInfo := policyContext.AdmissionInfo
	ctx := policyContext.Context

	resCache := policyContext.ResourceCache
	jsonContext := policyContext.JSONContext
	logger := log.Log.WithName("Generate").WithValues("policy", policy.Name, "kind", new.GetKind(), "namespace", new.GetNamespace(), "name", new.GetName())

	return filterRules(policy, new, old, admissionInfo, ctx, logger, policyContext.ExcludeGroupRole, resCache, jsonContext)
}

// filterRule checks if a rule matches the rule selection criteria.
//
func filterRule(rule kyverno.Rule, new, old unstructured.Unstructured, admissionInfo kyverno.RequestInfo, ctx context.EvalInterface, log logr.Logger, excludeGroupRole []string, resCache resourcecache.ResourceCacheIface, jsonContext *context.Context) *response.RuleResponse {
	if !rule.HasGenerate() {
		return nil
	}

	startTime := time.Now()

	if err := MatchesResourceDescription(new, rule, admissionInfo, excludeGroupRole); err != nil {
		if err := MatchesResourceDescription(old, rule, admissionInfo, excludeGroupRole); err == nil {
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
	if err := AddResourceToContext(log, rule.Context, resCache, jsonContext); err != nil {
		log.V(4).Info("cannot add configmaps to context", "reason", err.Error())
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

func filterRules(policy kyverno.ClusterPolicy, new, old unstructured.Unstructured, admissionInfo kyverno.RequestInfo, ctx context.EvalInterface, log logr.Logger, excludeGroupRole []string, resCache resourcecache.ResourceCacheIface, jsonContext *context.Context) response.EngineResponse {
	resp := response.EngineResponse{
		PolicyResponse: response.PolicyResponse{
			Policy: policy.Name,
			Resource: response.ResourceSpec{
				Kind:      new.GetKind(),
				Name:      new.GetName(),
				Namespace: new.GetNamespace(),
			},
		},
	}

	for _, rule := range policy.Spec.Rules {
		if ruleResp := filterRule(rule, new, old, admissionInfo, ctx, log, excludeGroupRole, resCache, jsonContext); ruleResp != nil {
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
		}
	}

	return resp
}
