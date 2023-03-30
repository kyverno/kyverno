package engine

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers/mutation"
	"github.com/kyverno/kyverno/pkg/engine/internal"
)

func (e *engine) verifyAndPatchImages(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
) (engineapi.EngineResponse, engineapi.ImageVerificationMetadata) {
	policy := policyContext.Policy()
	resp := engineapi.NewEngineResponseFromPolicyContext(policyContext, nil)
	matchedResource := policyContext.NewResource()

	startTime := time.Now()

	defer func() {
		internal.BuildResponse(policyContext, &resp, startTime)
		logger.V(4).Info("processed image verification rules",
			"time", resp.PolicyResponse.Stats.ProcessingTime.String(),
			"applied", resp.PolicyResponse.Stats.RulesAppliedCount,
			"successful", resp.IsSuccessful(),
		)
	}()

	policyContext.JSONContext().Checkpoint()
	defer policyContext.JSONContext().Restore()

	ivm := engineapi.ImageVerificationMetadata{}
	applyRules := policy.GetSpec().GetApplyRules()
	handler := mutation.NewMutateImageHandler(e.configuration, e.rclient, &ivm)
	for _, rule := range autogen.ComputeRules(policyContext.Policy()) {
		resource, ruleResp := e.invokeRuleHandler(ctx, logger, handler, policyContext, matchedResource, rule, engineapi.ImageVerify)
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
	internal.BuildResponse(policyContext, &resp, startTime)
	return resp, ivm
}
