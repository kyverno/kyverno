package engine

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/internal"
)

func (e *engine) validate(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
) engineapi.EngineResponse {
	startTime := time.Now()
	logger.V(4).Info("start validate policy processing", "startTime", startTime)
	policyResponse := e.validateResource(ctx, logger, policyContext)
	defer logger.V(4).Info("finished policy processing", "processingTime", policyResponse.Stats.ProcessingTime.String(), "validationRulesApplied", policyResponse.Stats.RulesAppliedCount)
	engineResponse := engineapi.NewEngineResponseFromPolicyContext(policyContext, nil)
	engineResponse.PolicyResponse = *policyResponse
	return *internal.BuildResponse(policyContext, &engineResponse, startTime)
}

func (e *engine) validateResource(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
) *engineapi.PolicyResponse {
	resp := &engineapi.PolicyResponse{}

	policyContext.JSONContext().Checkpoint()
	defer policyContext.JSONContext().Restore()

	applyRules := policyContext.Policy().GetSpec().GetApplyRules()

	for _, rule := range autogen.ComputeRules(policyContext.Policy()) {
		logger := internal.LoggerWithRule(logger, rule)
		startTime := time.Now()
		hasValidate := rule.HasValidate()
		hasValidateImage := rule.HasImagesValidationChecks()
		hasYAMLSignatureVerify := rule.HasYAMLSignatureVerify()
		if !hasValidate && !hasValidateImage {
			continue
		}
		var handler handlers.Handler
		if hasValidate && !hasYAMLSignatureVerify {
			handler = e.validateResourceHandler
		} else if hasValidateImage {
			handler = e.validateImageHandler
		} else if hasYAMLSignatureVerify {
			handler = e.validateManifestHandler
		}
		if handler != nil {
			_, ruleResp := e.invokeRuleHandler(ctx, logger, handler, policyContext, policyContext.NewResource(), rule, engineapi.Validation)
			for _, ruleResp := range ruleResp {
				ruleResp := ruleResp
				internal.AddRuleResponse(resp, &ruleResp, startTime)
				logger.V(4).Info("finished processing rule", "processingTime", ruleResp.Stats.ProcessingTime.String())
			}
		}
		if applyRules == kyvernov1.ApplyOne && resp.Stats.RulesAppliedCount > 0 {
			break
		}
	}
	return resp
}
