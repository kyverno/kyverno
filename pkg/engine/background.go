package engine

import (
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/engine/common"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/logging"
)

// ApplyBackgroundChecks checks for validity of generate and mutateExisting rules on the resource
// 1. validate variables to be substitute in the general ruleInfo (match,exclude,condition)
//   - the caller has to check the ruleResponse to determine whether the path exist
//
// 2. returns the list of rules that are applicable on this policy and resource, if 1 succeed
func ApplyBackgroundChecks(policyContext *PolicyContext) (resp *response.EngineResponse) {
	policyStartTime := time.Now()
	return filterRules(policyContext, policyStartTime)
}

func filterRules(policyContext *PolicyContext, startTime time.Time) *response.EngineResponse {
	kind := policyContext.newResource.GetKind()
	name := policyContext.newResource.GetName()
	namespace := policyContext.newResource.GetNamespace()
	apiVersion := policyContext.newResource.GetAPIVersion()
	resp := &response.EngineResponse{
		PolicyResponse: response.PolicyResponse{
			Policy: response.PolicySpec{
				Name:      policyContext.policy.GetName(),
				Namespace: policyContext.policy.GetNamespace(),
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

	if policyContext.excludeResourceFunc(kind, namespace, name) {
		logging.WithName("ApplyBackgroundChecks").Info("resource excluded", "kind", kind, "namespace", namespace, "name", name)
		return resp
	}

	applyRules := policyContext.policy.GetSpec().GetApplyRules()
	for _, rule := range autogen.ComputeRules(policyContext.policy) {
		if ruleResp := filterRule(rule, policyContext); ruleResp != nil {
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
			if applyRules == kyvernov1.ApplyOne && ruleResp.Status != response.RuleStatusSkip {
				break
			}
		}
	}

	return resp
}

func filterRule(rule kyvernov1.Rule, policyContext *PolicyContext) *response.RuleResponse {
	if !rule.HasGenerate() && !rule.IsMutateExisting() {
		return nil
	}

	ruleType := response.Mutation
	if rule.HasGenerate() {
		ruleType = response.Generation
	}

	var err error
	startTime := time.Now()

	policy := policyContext.policy
	newResource := policyContext.newResource
	oldResource := policyContext.oldResource
	// admissionInfo := policyContext.admissionInfo
	ctx := policyContext.jsonContext
	// excludeGroupRole := policyContext.excludeGroupRole
	// namespaceLabels := policyContext.namespaceLabels

	logger := logging.WithName(string(ruleType)).WithValues("policy", policy.GetName(),
		"kind", newResource.GetKind(), "namespace", newResource.GetNamespace(), "name", newResource.GetName())

	if err = MatchesResourceDescription(newResource, policyContext, rule, ""); err != nil {
		if ruleType == response.Generation {
			// if the oldResource matched, return "false" to delete GR for it
			if err = MatchesResourceDescription(oldResource, policyContext, rule, ""); err == nil {
				return &response.RuleResponse{
					Name:   rule.Name,
					Type:   ruleType,
					Status: response.RuleStatusFail,
					RuleStats: response.RuleStats{
						ProcessingTime:         time.Since(startTime),
						RuleExecutionTimestamp: startTime.Unix(),
					},
				}
			}
		}
		logger.V(4).Info("rule not matched", "reason", err.Error())
		return nil
	}

	policyContext.jsonContext.Checkpoint()
	defer policyContext.jsonContext.Restore()

	if err = LoadContext(logger, rule.Context, policyContext, rule.Name); err != nil {
		logger.V(4).Info("cannot add external data to the context", "reason", err.Error())
		return nil
	}

	ruleCopy := rule.DeepCopy()
	if after, err := variables.SubstituteAllInPreconditions(logger, ctx, ruleCopy.GetAnyAllConditions()); err != nil {
		logger.V(4).Info("failed to substitute vars in preconditions, skip current rule", "rule name", ruleCopy.Name)
		return nil
	} else {
		ruleCopy.SetAnyAllConditions(after)
	}

	// operate on the copy of the conditions, as we perform variable substitution
	copyConditions, err := common.TransformConditions(ruleCopy.GetAnyAllConditions())
	if err != nil {
		logger.V(4).Info("cannot copy AnyAllConditions", "reason", err.Error())
		return nil
	}

	// evaluate pre-conditions
	if !variables.EvaluateConditions(logger, ctx, copyConditions) {
		logger.V(4).Info("skip rule as preconditions are not met", "rule", ruleCopy.Name)
		return &response.RuleResponse{
			Name:   ruleCopy.Name,
			Type:   ruleType,
			Status: response.RuleStatusSkip,
			RuleStats: response.RuleStats{
				ProcessingTime:         time.Since(startTime),
				RuleExecutionTimestamp: startTime.Unix(),
			},
		}
	}

	// build rule Response
	return &response.RuleResponse{
		Name:   ruleCopy.Name,
		Type:   ruleType,
		Status: response.RuleStatusPass,
		RuleStats: response.RuleStats{
			ProcessingTime:         time.Since(startTime),
			RuleExecutionTimestamp: startTime.Unix(),
		},
	}
}
