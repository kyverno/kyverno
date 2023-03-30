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
) engineapi.PolicyResponse {
	resp := &engineapi.PolicyResponse{}

	policyContext.JSONContext().Checkpoint()
	defer policyContext.JSONContext().Restore()

	applyRules := policyContext.Policy().GetSpec().GetApplyRules()

	for _, rule := range autogen.ComputeRules(policyContext.Policy()) {
		logger := internal.LoggerWithRule(logger, rule)
		startTime := time.Now()
		hasValidate := rule.HasValidate()
		hasVerifyImageChecks := rule.HasVerifyImageChecks()
		if !hasValidate && !hasVerifyImageChecks {
			continue
		}
		var handler handlers.Handler
		if hasValidate {
			hasVerifyManifest := rule.HasVerifyManifests()
			hasValidatePss := rule.HasValidatePodSecurity()
			if hasVerifyManifest {
				handler = e.validateManifestHandler
			} else if hasValidatePss {
				handler = e.validatePssHandler
			} else {
				handler = e.validateResourceHandler
			}
		} else if hasVerifyImageChecks {
			handler = e.validateImageHandler
		}
		if handler != nil {
			_, ruleResp := e.invokeRuleHandler(
				ctx,
				logger,
				WithHandler(handler),
				policyContext,
				policyContext.NewResource(),
				rule,
				engineapi.Validation,
			)
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
	return *resp
}
