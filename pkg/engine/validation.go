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

	gvk, _ := policyContext.ResourceKind()
	for _, rule := range autogen.Default.ComputeRules(policy, gvk.Kind) {
		startTime := time.Now()
		logger := internal.LoggerWithRule(logger, rule)
		handlerFactory := func() (handlers.Handler, error) {
			hasValidate := rule.HasValidate()
			hasVerifyImageChecks := rule.HasVerifyImageChecks()
			if !hasValidate && !hasVerifyImageChecks {
				return nil, nil
			}
			if hasValidate {
				hasValidateAssert := rule.HasValidateAssert()
				if hasValidateAssert && rule.Validation.Assert.Value != nil {
					return validation.NewValidateAssertHandler(e.client, e.isCluster)
				}
				hasVerifyManifest := rule.HasVerifyManifests()
				hasValidatePss := rule.HasValidatePodSecurity()
				hasValidateCEL := rule.HasValidateCEL()
				if hasVerifyManifest {
					return validation.NewValidateManifestHandler(
						policyContext,
						e.client,
						e.isCluster,
					)
				} else if hasValidatePss {
					return validation.NewValidatePssHandler(e.client, e.isCluster)
				} else if hasValidateCEL {
					return validation.NewValidateCELHandler(e.client, e.isCluster)
				} else {
					return validation.NewValidateResourceHandler(e.client, e.isCluster)
				}
			} else if hasVerifyImageChecks {
				return validation.NewValidateImageHandler(
					policyContext,
					policyContext.NewResource(),
					rule,
					e.configuration,
					e.client,
					e.isCluster,
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
		// Stamp each validate rule response with its effective failureAction so the
		// admission block decision honors each rule's action independently instead of
		// collapsing the whole policy to a single action (issue #16557).
		if rule.HasValidate() {
			action := engineapi.ResolveValidationFailureAction(rule.Validation, policy.GetSpec(), policyContext.NamespaceLabels(), matchedResource.GetNamespace())
			for i := range ruleResp {
				ruleResp[i] = *ruleResp[i].WithValidationFailureAction(action)
			}
		}
		resp.Add(engineapi.NewExecutionStats(startTime, time.Now()), ruleResp...)
		if applyRules == kyvernov1.ApplyOne && resp.RulesAppliedCount() > 0 {
			break
		}
	}
	return resp
}
