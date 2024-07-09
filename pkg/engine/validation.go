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
	enforce bool,
) engineapi.PolicyResponse {
	resp := engineapi.NewPolicyResponse()
	policy := policyContext.Policy()
	matchedResource := policyContext.NewResource()
	applyRules := policy.GetSpec().GetApplyRules()

	policyContext.JSONContext().Checkpoint()
	defer policyContext.JSONContext().Restore()

	hasEnforce := func(actions []kyvernov1.ValidationFailureActionOverride) bool {
		for _, action := range actions {
			if enforce && action.Action.Enforce() {
				return true
			}
		}
		return false
	}
	gvk, _ := policyContext.ResourceKind()
	for _, rule := range autogen.ComputeRules(policy, gvk.Kind) {
		isEnforce := false
		// check the validation failure action
		validationFailureAction := rule.Validation.ValidationFailureAction
		if validationFailureAction == nil {
			validationFailureAction = &policy.GetSpec().ValidationFailureAction
		}
		if enforce && validationFailureAction.Enforce() {
			isEnforce = true
		}
		// check the validation failure action overrides
		if len(rule.Validation.ValidationFailureActionOverrides) != 0 {
			if hasEnforce(rule.Validation.ValidationFailureActionOverrides) {
				isEnforce = true
			}
		} else if len(policy.GetSpec().ValidationFailureActionOverrides) != 0 {
			if hasEnforce(policy.GetSpec().ValidationFailureActionOverrides) {
				isEnforce = true
			}
		}
		if isEnforce == enforce {
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
					hasValidateCEL := rule.HasValidateCEL()
					if hasVerifyManifest {
						return validation.NewValidateManifestHandler(
							policyContext,
							e.client,
						)
					} else if hasValidatePss {
						return validation.NewValidatePssHandler()
					} else if hasValidateCEL {
						return validation.NewValidateCELHandler(e.client)
					} else {
						return validation.NewValidateResourceHandler()
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
			resp.Add(engineapi.NewExecutionStats(startTime, time.Now()), ruleResp...)
			if applyRules == kyvernov1.ApplyOne && resp.RulesAppliedCount() > 0 {
				break
			}
		}
	}
	return resp
}
