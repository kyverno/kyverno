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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (e *engine) verifyAndPatchImages(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
) (engineapi.PolicyResponse, unstructured.Unstructured, engineapi.ImageVerificationMetadata) {
	resp := engineapi.NewPolicyResponse()
	policy := policyContext.Policy()
	matchedResource := policyContext.NewResource()
	applyRules := policy.GetSpec().GetApplyRules()
	ivm := engineapi.ImageVerificationMetadata{}

	policyContext.JSONContext().Checkpoint()
	defer policyContext.JSONContext().Restore()

	for _, rule := range autogen.Default.ComputeRules(policy, "") {
		startTime := time.Now()
		logger := internal.LoggerWithRule(logger, rule)
		handlerFactory := func() (handlers.Handler, error) {
			if !rule.HasVerifyImages() {
				return nil, nil
			}
			return mutation.NewMutateImageHandler(
				policyContext,
				matchedResource,
				rule,
				e.configuration,
				e.rclientFactory,
				e.ivCache,
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
		resp.Add(engineapi.NewExecutionStats(startTime, time.Now()), ruleResp...)
		if applyRules == kyvernov1.ApplyOne && resp.RulesAppliedCount() > 0 {
			break
		}
	}
	return resp, matchedResource, ivm
}
