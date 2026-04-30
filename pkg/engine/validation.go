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

// assertOnlyValidation returns true when the Validation struct has only the
// deprecated Assert field set and no actionable validation content.
func assertOnlyValidation(v *kyvernov1.Validation) bool {
	return v != nil && v.Assert != nil &&
		v.RawPattern == nil &&
		v.RawAnyPattern == nil &&
		v.Deny == nil &&
		len(v.ForEachValidation) == 0 &&
		v.Manifests == nil &&
		v.PodSecurity == nil &&
		v.CEL == nil
}

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
				if assertOnlyValidation(rule.Validation) {
					// Assert is deprecated and has no effect; skip rules with only Assert set
					return nil, nil
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
		resp.Add(engineapi.NewExecutionStats(startTime, time.Now()), ruleResp...)
		if applyRules == kyvernov1.ApplyOne && resp.RulesAppliedCount() > 0 {
			break
		}
	}
	return resp
}
