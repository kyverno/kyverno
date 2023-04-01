package engine

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/handlers/mutation"
	"github.com/kyverno/kyverno/pkg/engine/internal"
)

func (e *engine) verifyAndPatchImages(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
) (engineapi.PolicyResponse, engineapi.ImageVerificationMetadata) {
	resp := engineapi.NewPolicyResponse()

	policyContext.JSONContext().Checkpoint()
	defer policyContext.JSONContext().Restore()

	ivm := engineapi.ImageVerificationMetadata{}
	policy := policyContext.Policy()
	matchedResource := policyContext.NewResource()
	applyRules := policy.GetSpec().GetApplyRules()
	for _, rule := range autogen.ComputeRules(policyContext.Policy()) {
		if rule.HasVerifyImages() {
			startTime := time.Now()
			handlerFactory := func() (handlers.Handler, error) {
				return mutation.NewMutateImageHandler(
					policyContext,
					matchedResource,
					rule,
					e.configuration,
					e.rclient,
					&ivm,
				)
			}
			resource, ruleResp := e.invokeRuleHandler(
				ctx,
				logger,
				handlerFactory,
				policyContext,
				matchedResource,
				rule,
				engineapi.ImageVerify,
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
	}
	// TODO: i doesn't make sense to not return the patched resource here
	return resp, ivm
}
