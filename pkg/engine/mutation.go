package engine

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
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

	policyContext.JSONContext().Checkpoint()
	defer policyContext.JSONContext().Restore()

	applyRules := policy.GetSpec().GetApplyRules()

	for _, rule := range autogen.ComputeRules(policy) {
		logger := internal.LoggerWithRule(logger, rule)
		if !rule.HasMutate() {
			continue
		}
		handlerFactory := handlers.WithHandler(e.mutateResourceHandler)
		if !policyContext.AdmissionOperation() && rule.IsMutateExisting() {
			handlerFactory = handlers.WithHandler(e.mutateExistingHandler)
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
		for _, ruleResp := range ruleResp {
			resp.Add(ruleResp)
		}
		if applyRules == kyvernov1.ApplyOne && resp.Stats.RulesAppliedCount > 0 {
			break
		}
	}
	return resp, matchedResource
}
