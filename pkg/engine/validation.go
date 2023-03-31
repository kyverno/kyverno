package engine

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/handlers/validation"
	"github.com/kyverno/kyverno/pkg/engine/internal"
)

func (e *engine) validate(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
) engineapi.PolicyResponse {
	resp := engineapi.NewPolicyResponse()
	policy := policyContext.Policy()
	matchedResource := policyContext.NewResource()
	applyRules := policy.GetSpec().GetApplyRules()

	policyContext.JSONContext().Checkpoint()
	defer policyContext.JSONContext().Restore()

	for _, rule := range autogen.ComputeRules(policy) {
		startTime := time.Now()
		logger := internal.LoggerWithRule(logger, rule)
		handlerFactory := func() (handlers.Handler, error) {
			hasValidate := rule.HasValidate()
			hasVerifyImageChecks := rule.HasVerifyImageChecks()
			if !hasValidate && !hasVerifyImageChecks {
				return nil, nil
			}
			if hasValidate {
				hasVerifyManifest := rule.HasVerifyManifests()
				hasValidatePss := rule.HasValidatePodSecurity()
				if hasVerifyManifest {
					return e.validateManifestHandler, nil
				} else if hasValidatePss {
					return e.validatePssHandler, nil
				} else {
					return e.validateResourceHandler, nil
				}
			} else if hasVerifyImageChecks {
				return validation.NewValidateImageHandler(
					policyContext,
					policyContext.NewResource(),
					rule,
					e.configuration,
				)
			}
			return nil, nil
		}
		resource, ruleResp := e.invokeRuleHandler(
			ctx,
			logger,
			handlerFactory,
			policyContext,
			matchedResource,
			rule,
			engineapi.Validation,
		)
		matchedResource = resource
		for _, ruleResp := range ruleResp {
			ruleResp := ruleResp
			internal.AddRuleResponse(&resp, &ruleResp, startTime)
			logger.V(4).Info("finished processing rule", "processingTime", ruleResp.Stats.ProcessingTime.String())
		}
		if applyRules == kyvernov1.ApplyOne && resp.Stats.RulesAppliedCount > 0 {
			break
		}
	}
	return resp
}
