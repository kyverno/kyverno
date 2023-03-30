package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/trace"
)

func (e *engine) verifyAndPatchImages(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
) (engineapi.EngineResponse, engineapi.ImageVerificationMetadata) {
	policy := policyContext.Policy()
	resp := engineapi.NewEngineResponseFromPolicyContext(policyContext, nil)

	startTime := time.Now()

	defer func() {
		internal.BuildResponse(policyContext, &resp, startTime)
		logger.V(4).Info("processed image verification rules",
			"time", resp.PolicyResponse.Stats.ProcessingTime.String(),
			"applied", resp.PolicyResponse.Stats.RulesAppliedCount,
			"successful", resp.IsSuccessful(),
		)
	}()

	policyContext.JSONContext().Checkpoint()
	defer policyContext.JSONContext().Restore()

	ivm := engineapi.ImageVerificationMetadata{}
	applyRules := policy.GetSpec().GetApplyRules()

	for _, rule := range autogen.ComputeRules(policyContext.Policy()) {
		tracing.ChildSpan(
			ctx,
			"pkg/engine",
			fmt.Sprintf("RULE %s", rule.Name),
			func(ctx context.Context, span trace.Span) {
				e.doVerifyAndPatch(ctx, logger, policyContext, rule, &resp, &ivm)
			},
		)

		if applyRules == kyvernov1.ApplyOne && resp.PolicyResponse.Stats.RulesAppliedCount > 0 {
			break
		}
	}
	internal.BuildResponse(policyContext, &resp, startTime)
	return resp, ivm
}

func (e *engine) doVerifyAndPatch(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	rule kyvernov1.Rule,
	resp *engineapi.EngineResponse,
	ivm *engineapi.ImageVerificationMetadata,
) {
	if len(rule.VerifyImages) == 0 {
		return
	}
	startTime := time.Now()
	logger = internal.LoggerWithRule(logger, rule)

	if err := matches(rule, policyContext, policyContext.NewResource()); err != nil {
		logger.V(5).Info("resource does not match rule", "reason", err.Error())
		return
	}

	// check if there is a corresponding policy exception
	ruleResp := e.hasPolicyExceptions(logger, engineapi.ImageVerify, policyContext, rule)
	if ruleResp != nil {
		resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
		return
	}

	logger.V(3).Info("processing image verification rule")

	ruleImages, _, err := engineutils.ExtractMatchingImages(
		policyContext.NewResource(),
		policyContext.JSONContext(),
		rule,
		e.configuration,
	)
	if err != nil {
		internal.AddRuleResponse(
			&resp.PolicyResponse,
			internal.RuleError(rule, engineapi.ImageVerify, "failed to extract images", err),
			startTime,
		)
		return
	}
	if len(ruleImages) == 0 {
		return
	}
	policyContext.JSONContext().Restore()
	if err := internal.LoadContext(ctx, e, policyContext, rule); err != nil {
		internal.AddRuleResponse(
			&resp.PolicyResponse,
			internal.RuleError(rule, engineapi.ImageVerify, "failed to load context", err),
			startTime,
		)
		return
	}
	ruleCopy, err := substituteVariables(&rule, policyContext.JSONContext(), logger)
	if err != nil {
		internal.AddRuleResponse(
			&resp.PolicyResponse,
			internal.RuleError(rule, engineapi.ImageVerify, "failed to substitute variables", err),
			startTime,
		)
		return
	}
	iv := internal.NewImageVerifier(
		logger,
		e.rclient,
		policyContext,
		*ruleCopy,
		ivm,
	)
	for _, imageVerify := range ruleCopy.VerifyImages {
		for _, r := range iv.Verify(ctx, imageVerify, ruleImages, e.configuration) {
			internal.AddRuleResponse(&resp.PolicyResponse, r, startTime)
		}
	}
}

func substituteVariables(rule *kyvernov1.Rule, ctx enginecontext.EvalInterface, logger logr.Logger) (*kyvernov1.Rule, error) {
	// remove attestations as variables are not substituted in them
	ruleCopy := *rule.DeepCopy()
	for i := range ruleCopy.VerifyImages {
		ruleCopy.VerifyImages[i].Attestations = nil
	}

	var err error
	ruleCopy, err = variables.SubstituteAllInRule(logger, ctx, ruleCopy)
	if err != nil {
		return nil, err
	}

	// replace attestations
	for i := range rule.VerifyImages {
		ruleCopy.VerifyImages[i].Attestations = rule.VerifyImages[i].Attestations
	}

	return &ruleCopy, nil
}
