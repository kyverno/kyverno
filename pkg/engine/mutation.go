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

// mutate performs mutation. Overlay first and then mutation patches
func (e *engine) mutate(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
) (engineapi.PolicyResponse, unstructured.Unstructured) {
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
			if !rule.HasMutate() {
				return nil, nil
			}
			if !policyContext.AdmissionOperation() && rule.IsMutateExisting() {
				return mutation.NewMutateExistingHandler(e.client)
			}
			return mutation.NewMutateResourceHandler()
		}
		resource, ruleResp := e.invokeRuleHandler(
			ctx,
			logger,
			handlerFactory,
			policyContext,
			matchedResource,
			rule,
			engineapi.Mutation,
		)
		matchedResource = resource
		resp.Add(engineapi.NewExecutionStats(startTime, time.Now()), ruleResp...)
		if applyRules == kyvernov1.ApplyOne && resp.RulesAppliedCount() > 0 {
			break
		}
	}
	return resp, matchedResource
}
