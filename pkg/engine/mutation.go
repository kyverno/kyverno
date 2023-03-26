package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Mutate performs mutation. Overlay first and then mutation patches
func (e *engine) mutate(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
) engineapi.EngineResponse {
	startTime := time.Now()
	resp := engineapi.NewEngineResponseFromPolicyContext(policyContext, nil)
	policy := policyContext.Policy()
	matchedResource := policyContext.NewResource()

	startMutateResultResponse(&resp, policy, matchedResource)

	policyContext.JSONContext().Checkpoint()
	defer policyContext.JSONContext().Restore()

	applyRules := policy.GetSpec().GetApplyRules()

	for _, rule := range autogen.ComputeRules(policy) {
		if !rule.HasMutate() {
			continue
		}
		resource, ruleResp := tracing.ChildSpan2(
			ctx,
			"pkg/engine",
			fmt.Sprintf("RULE %s", rule.Name),
			func(ctx context.Context, span trace.Span) (unstructured.Unstructured, []engineapi.RuleResponse) {
				logger := internal.LoggerWithRule(logger, rule)
				polexFilter := func(logger logr.Logger, policyContext engineapi.PolicyContext, rule kyvernov1.Rule) *engineapi.RuleResponse {
					return hasPolicyExceptions(logger, engineapi.Validation, e.exceptionSelector, policyContext, &rule, e.configuration)
				}
				if !policyContext.AdmissionOperation() && rule.IsMutateExisting() {
					return e.mutateExistingHandler.Process(ctx, logger, policyContext, matchedResource, rule, polexFilter)
				} else {
					return e.mutateHandler.Process(ctx, logger, policyContext, matchedResource, rule, polexFilter)
				}
			},
		)
		matchedResource = resource
		for _, ruleResp := range ruleResp {
			internal.AddRuleResponse(&resp.PolicyResponse, &ruleResp, startTime)
			logger.V(4).Info("finished processing rule", "processingTime", ruleResp.Stats.ProcessingTime.String())
		}
		if applyRules == kyvernov1.ApplyOne && resp.PolicyResponse.Stats.RulesAppliedCount > 0 {
			break
		}
	}
	resp.PatchedResource = matchedResource
	endMutateResultResponse(logger, &resp, startTime)
	return resp
}

func startMutateResultResponse(resp *engineapi.EngineResponse, policy kyvernov1.PolicyInterface, resource unstructured.Unstructured) {
	if resp == nil {
		return
	}
}

func endMutateResultResponse(logger logr.Logger, resp *engineapi.EngineResponse, startTime time.Time) {
	if resp == nil {
		return
	}
	resp.PolicyResponse.Stats.ProcessingTime = time.Since(startTime)
	resp.PolicyResponse.Stats.Timestamp = startTime.Unix()
	logger.V(5).Info("finished processing policy", "processingTime", resp.PolicyResponse.Stats.ProcessingTime.String(), "mutationRulesApplied", resp.PolicyResponse.Stats.RulesAppliedCount)
}
