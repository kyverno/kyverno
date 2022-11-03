package engine

import (
	"time"

	"github.com/go-logr/logr"
	gojmespath "github.com/jmespath/go-jmespath"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
)

type cleaner struct {
	log              logr.Logger
	ctx              *PolicyContext
	rule             *kyvernov1.Rule
	contextEntries   []kyvernov1.ContextEntry
	anyAllConditions apiextensions.JSON
}

func newCleaner(log logr.Logger, ctx *PolicyContext) *cleaner {
	rule := ctx.Policy.GetSpec().Rules[0]
	ruleCopy := rule.DeepCopy()
	return &cleaner{
		log:              log,
		rule:             ruleCopy,
		ctx:              ctx,
		contextEntries:   ruleCopy.Context,
		anyAllConditions: ruleCopy.GetAnyAllConditions(),
	}
}

// CleanUp applies cleanup rules from policy on the resource
func Cleanup(policyContext *PolicyContext) (resp *response.EngineResponse) {
	resp = &response.EngineResponse{}
	startTime := time.Now()

	logger := buildLogger(policyContext)
	logger.V(4).Info("start cleanup policy processing", "startTime", startTime)
	defer func() {
		buildResponse(policyContext, resp, startTime)
		logger.V(4).Info("finished policy processing", "processingTime", resp.PolicyResponse.ProcessingTime.String(), "cleanupRulesApplied", resp.PolicyResponse.RulesAppliedCount)
	}()

	resp = cleanupResource(logger, policyContext)
	return
}

func cleanupResource(log logr.Logger, ctx *PolicyContext) *response.EngineResponse {
	resp := &response.EngineResponse{}
	rule := ctx.Policy.GetSpec().Rules[0]
	if matches(log, &rule, ctx) {
		startTime := time.Now()

		ruleResp := processCleanupRule(log, ctx, &rule)

		if ruleResp != nil {
			addRuleResponse(log, resp, ruleResp, startTime)
		}
	}

	return resp
}

func processCleanupRule(log logr.Logger, ctx *PolicyContext, rule *kyvernov1.Rule) *response.RuleResponse {
	v := newCleaner(log, ctx)
	return v.cleanup()
}

func (v *cleaner) cleanup() *response.RuleResponse {
	if err := v.loadContext(); err != nil {
		return ruleError(v.rule, response.CleanUp, "failed to load context", err)
	}

	preconditionsPassed, err := checkPreconditions(v.log, v.ctx, v.anyAllConditions)
	if err != nil {
		return ruleError(v.rule, response.CleanUp, "failed to evaluate preconditions", err)
	}

	if !preconditionsPassed {
		return ruleResponse(*v.rule, response.CleanUp, "preconditions not met", response.RuleStatusSkip, nil)
	}

	ruleResponse := v.checkCleanupConditions()
	return ruleResponse
}

func (v *cleaner) checkCleanupConditions() *response.RuleResponse {
	if len(v.rule.CleanUp.Conditions.AllConditions) != 0 || len(v.rule.CleanUp.Conditions.AnyConditions) != 0 {
		cleanupConditionsPassed, err := checkPreconditions(v.log, v.ctx, v.rule.CleanUp.Conditions)
		if err != nil {
			return ruleError(v.rule, response.CleanUp, "failed to evaluate cleanup Conditions", err)
		}
		if !cleanupConditionsPassed {
			return ruleResponse(*v.rule, response.CleanUp, "cleanup conditions not met", response.RuleStatusSkip, nil)
		}
	}

	return ruleResponse(*v.rule, response.CleanUp, "cleanup conditions met", response.RuleStatusPass, nil)
}

func (v *cleaner) loadContext() error {
	if err := LoadContext(v.log, v.contextEntries, v.ctx, v.rule.Name); err != nil {
		if _, ok := err.(gojmespath.NotFoundError); ok {
			v.log.V(3).Info("failed to load context", "reason", err.Error())
		} else {
			v.log.Error(err, "failed to load context")
		}

		return err
	}

	return nil
}
