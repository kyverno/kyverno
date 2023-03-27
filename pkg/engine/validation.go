package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/trace"
)

func (e *engine) validate(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
) engineapi.EngineResponse {
	startTime := time.Now()
	logger.V(4).Info("start validate policy processing", "startTime", startTime)
	policyResponse := e.validateResource(ctx, logger, policyContext)
	defer logger.V(4).Info("finished policy processing", "processingTime", policyResponse.Stats.ProcessingTime.String(), "validationRulesApplied", policyResponse.Stats.RulesAppliedCount)
	engineResponse := engineapi.NewEngineResponseFromPolicyContext(policyContext, nil)
	engineResponse.PolicyResponse = *policyResponse
	return *internal.BuildResponse(policyContext, &engineResponse, startTime)
}

func (e *engine) validateResource(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
) *engineapi.PolicyResponse {
	resp := &engineapi.PolicyResponse{}

	policyContext.JSONContext().Checkpoint()
	defer policyContext.JSONContext().Restore()

	rules := autogen.ComputeRules(policyContext.Policy())
	matchCount := 0
	applyRules := policyContext.Policy().GetSpec().GetApplyRules()

	for i := range rules {
		rule := &rules[i]
		logger := internal.LoggerWithRule(logger, rules[i])
		logger.V(3).Info("processing validation rule", "matchCount", matchCount)
		policyContext.JSONContext().Reset()
		startTime := time.Now()
		ruleResp := tracing.ChildSpan1(
			ctx,
			"pkg/engine",
			fmt.Sprintf("RULE %s", rule.Name),
			func(ctx context.Context, span trace.Span) []engineapi.RuleResponse {
				hasValidate := rule.HasValidate()
				hasValidateImage := rule.HasImagesValidationChecks()
				hasYAMLSignatureVerify := rule.HasYAMLSignatureVerify()
				if !hasValidate && !hasValidateImage {
					return nil
				}
				if !matches(logger, rule, policyContext, e.configuration) {
					return nil
				}
				// check if there is a corresponding policy exception
				ruleResp := hasPolicyExceptions(logger, engineapi.Validation, e.exceptionSelector, policyContext, *rule, e.configuration)
				if ruleResp != nil {
					return handlers.RuleResponses(ruleResp)
				}
				policyContext.JSONContext().Reset()
				if hasValidate && !hasYAMLSignatureVerify {
					_, rr := e.validateHandler.Process(
						ctx,
						logger,
						policyContext,
						policyContext.NewResource(),
						*rule,
					)
					return rr
				} else if hasValidateImage {
					_, rr := e.validateImageHandler.Process(
						ctx,
						logger,
						policyContext,
						policyContext.NewResource(),
						*rule,
					)
					return rr
				} else if hasYAMLSignatureVerify {
					_, rr := e.verifyManifestHandler.Process(
						ctx,
						logger,
						policyContext,
						policyContext.NewResource(),
						*rule,
					)
					return rr
				}
				return nil
			},
		)
		for _, ruleResp := range ruleResp {
			ruleResp := ruleResp
			internal.AddRuleResponse(resp, &ruleResp, startTime)
			logger.V(4).Info("finished processing rule", "processingTime", ruleResp.Stats.ProcessingTime.String())
		}
		if applyRules == kyvernov1.ApplyOne && resp.Stats.RulesAppliedCount > 0 {
			break
		}
	}

	return resp
}

// matches checks if either the new or old resource satisfies the filter conditions defined in the rule
func matches(
	logger logr.Logger,
	rule *kyvernov1.Rule,
	ctx engineapi.PolicyContext,
	cfg config.Configuration,
) bool {
	gvk, subresource := ctx.ResourceKind()
	err := engineutils.MatchesResourceDescription(
		ctx.NewResource(),
		*rule,
		ctx.AdmissionInfo(),
		cfg.GetExcludedGroups(),
		ctx.NamespaceLabels(),
		"",
		gvk,
		subresource,
	)
	if err == nil {
		return true
	}
	oldResource := ctx.OldResource()
	if oldResource.Object != nil {
		err := engineutils.MatchesResourceDescription(
			ctx.OldResource(),
			*rule,
			ctx.AdmissionInfo(),
			cfg.GetExcludedGroups(),
			ctx.NamespaceLabels(),
			"",
			gvk,
			subresource,
		)
		if err == nil {
			return true
		}
	}
	logger.V(5).Info("resource does not match rule", "reason", err.Error())
	return false
}
