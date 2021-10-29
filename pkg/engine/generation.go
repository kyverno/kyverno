package engine

import (
	"time"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Generate checks for validity of generate rule on the resource
// 1. validate variables to be substitute in the general ruleInfo (match,exclude,condition)
//    - the caller has to check the ruleResponse to determine whether the path exist
// 2. returns the list of rules that are applicable on this policy and resource, if 1 succeed
func Generate(policyContext *PolicyContext) (resp *response.EngineResponse) {
	policyStartTime := time.Now()
	return filterRules(policyContext, policyStartTime)
}

func filterRules(policyContext *PolicyContext, startTime time.Time) *response.EngineResponse {
	kind := policyContext.NewResource.GetKind()
	name := policyContext.NewResource.GetName()
	namespace := policyContext.NewResource.GetNamespace()
	apiVersion := policyContext.NewResource.GetAPIVersion()
	resp := &response.EngineResponse{
		PolicyResponse: response.PolicyResponse{
			Policy: response.PolicySpec{
				Name:      policyContext.Policy.GetName(),
				Namespace: policyContext.Policy.GetNamespace(),
			},
			PolicyStats: response.PolicyStats{
				PolicyExecutionTimestamp: startTime.Unix(),
			},
			Resource: response.ResourceSpec{
				Kind:       kind,
				Name:       name,
				Namespace:  namespace,
				APIVersion: apiVersion,
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

func filterRule(rule kyverno.Rule, policyContext *PolicyContext) *response.RuleResponse {
	if !rule.HasGenerate() {
		return nil
	}

	var err error
	startTime := time.Now()

	policy := policyContext.Policy
	newResource := policyContext.NewResource
	oldResource := policyContext.OldResource
	admissionInfo := policyContext.AdmissionInfo
	ctx := policyContext.JSONContext
	resCache := policyContext.ResourceCache
	excludeGroupRole := policyContext.ExcludeGroupRole
	namespaceLabels := policyContext.NamespaceLabels

	logger := log.Log.WithName("Generate").WithValues("policy", policy.Name,
		"kind", newResource.GetKind(), "namespace", newResource.GetNamespace(), "name", newResource.GetName())

	if err = MatchesResourceDescription(newResource, rule, admissionInfo, excludeGroupRole, namespaceLabels, ""); err != nil {

		// if the oldResource matched, return "false" to delete GR for it
		if err = MatchesResourceDescription(oldResource, rule, admissionInfo, excludeGroupRole, namespaceLabels, ""); err == nil {
			return &response.RuleResponse{
				Name:   rule.Name,
				Type:   "Generation",
				Status: response.RuleStatusFail,
				RuleStats: response.RuleStats{
					ProcessingTime:         time.Since(startTime),
					RuleExecutionTimestamp: startTime.Unix(),
				},
			}
		}

		return nil
	}

	policyContext.JSONContext.Checkpoint()
	defer policyContext.JSONContext.Restore()

	if err = LoadContext(logger, rule.Context, resCache, policyContext, rule.Name); err != nil {
		logger.V(4).Info("cannot add external data to the context", "reason", err.Error())
		return nil
	}

	ruleCopy := rule.DeepCopy()
	ruleCopy.AnyAllConditions, err = variables.SubstituteAllInPreconditions(logger, ctx, ruleCopy.AnyAllConditions)
	if err != nil {
		logger.V(4).Info("failed to substitute vars in preconditions, skip current rule", "rule name", ruleCopy.Name)
		return nil
	}

	// operate on the copy of the conditions, as we perform variable substitution
	copyConditions, err := transformConditions(ruleCopy.AnyAllConditions)
	if err != nil {
		logger.V(4).Info("cannot copy AnyAllConditions", "reason", err.Error())
		return nil
	}

	// evaluate pre-conditions
	if !variables.EvaluateConditions(logger, ctx, copyConditions) {
		logger.V(4).Info("preconditions not satisfied, skipping rule", "rule", ruleCopy.Name)
		return nil
	}

	// build rule Response
	return &response.RuleResponse{
		Name:   ruleCopy.Name,
		Type:   "Generation",
		Status: response.RuleStatusPass,
		RuleStats: response.RuleStats{
			ProcessingTime:         time.Since(startTime),
			RuleExecutionTimestamp: startTime.Unix(),
		},
	}
}
