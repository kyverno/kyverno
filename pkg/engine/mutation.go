package engine

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Mutate performs mutation. Overlay first and then mutation patches
func (e *engine) mutate(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
) engineapi.EngineResponse {
	startTime := time.Now()
	resp := engineapi.NewEngineResponseFromPolicyContext(policyContext, nil)
	policy := policyContext.Policy()
	matchedResource := policyContext.NewResource()

	startMutateResultResponse(&resp, policy, matchedResource)

	policyContext.JSONContext().Checkpoint()
	defer policyContext.JSONContext().Restore()

	applyRules := policy.GetSpec().GetApplyRules()

	for _, rule := range autogen.ComputeRules(policy) {
		logger := internal.LoggerWithRule(logger, rule)
		if !rule.HasMutate() {
			continue
		}
		polexFilter := func(logger logr.Logger, policyContext engineapi.PolicyContext, rule kyvernov1.Rule) *engineapi.RuleResponse {
			return hasPolicyExceptions(logger, engineapi.Mutation, e.exceptionSelector, policyContext, rule, e.configuration)
		}
		handler := e.mutateResourceHandler
		if !policyContext.AdmissionOperation() && rule.IsMutateExisting() {
			handler = e.mutateExistingHandler
		}
		resource, ruleResp := e.invokeRuleHandler(ctx, logger, handler, policyContext, matchedResource, rule, polexFilter)
		matchedResource = resource
		for _, ruleResp := range ruleResp {
			ruleResp := ruleResp
			internal.AddRuleResponse(&resp.PolicyResponse, &ruleResp, startTime)
			logger.V(4).Info("finished processing rule", "processingTime", ruleResp.Stats.ProcessingTime.String())
		}
		if applyRules == kyvernov1.ApplyOne && resp.PolicyResponse.Stats.RulesAppliedCount > 0 {
			break
		}
	}
	resp.PatchedResource = matchedResource
	endMutateResultResponse(logger, &resp, startTime)
	return resp
}

func startMutateResultResponse(resp *engineapi.EngineResponse, policy kyvernov1.PolicyInterface, resource unstructured.Unstructured) {
	if resp == nil {
		return
	}
}

func endMutateResultResponse(logger logr.Logger, resp *engineapi.EngineResponse, startTime time.Time) {
	if resp == nil {
		return
	}
	resp.PolicyResponse.Stats.ProcessingTime = time.Since(startTime)
	resp.PolicyResponse.Stats.Timestamp = startTime.Unix()
	logger.V(5).Info("finished processing policy", "processingTime", resp.PolicyResponse.Stats.ProcessingTime.String(), "mutationRulesApplied", resp.PolicyResponse.Stats.RulesAppliedCount)
}
