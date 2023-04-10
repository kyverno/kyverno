package engine

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/variables"
)

// ApplyBackgroundChecks checks for validity of generate and mutateExisting rules on the resource
// 1. validate variables to be substitute in the general ruleInfo (match,exclude,condition)
//   - the caller has to check the ruleResponse to determine whether the path exist
//
// 2. returns the list of rules that are applicable on this policy and resource, if 1 succeed
func (e *engine) applyBackgroundChecks(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
) engineapi.PolicyResponse {
	return e.filterRules(policyContext, logger, time.Now())
}

func (e *engine) filterRules(
	policyContext engineapi.PolicyContext,
	logger logr.Logger,
	startTime time.Time,
) engineapi.PolicyResponse {
	policy := policyContext.Policy()
	resp := engineapi.NewPolicyResponse()
	applyRules := policy.GetSpec().GetApplyRules()
	for _, rule := range autogen.ComputeRules(policy) {
		logger := internal.LoggerWithRule(logger, rule)
		if ruleResp := e.filterRule(rule, logger, policyContext); ruleResp != nil {
			resp.Rules = append(resp.Rules, *ruleResp)
			if applyRules == kyvernov1.ApplyOne && ruleResp.Status() != engineapi.RuleStatusSkip {
				break
			}
		}
	}
	return resp
}

func (e *engine) filterRule(
	rule kyvernov1.Rule,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
) *engineapi.RuleResponse {
	if !rule.HasGenerate() && !rule.IsMutateExisting() {
		return nil
	}

	ruleType := engineapi.Mutation
	if rule.HasGenerate() {
		ruleType = engineapi.Generation
	}

	// check if there is a corresponding policy exception
	ruleResp := e.hasPolicyExceptions(logger, ruleType, policyContext, rule)
	if ruleResp != nil {
		return ruleResp
	}

	newResource := policyContext.NewResource()
	oldResource := policyContext.OldResource()
	admissionInfo := policyContext.AdmissionInfo()
	ctx := policyContext.JSONContext()
	namespaceLabels := policyContext.NamespaceLabels()
	policy := policyContext.Policy()
	gvk, subresource := policyContext.ResourceKind()

	if err := engineutils.MatchesResourceDescription(newResource, rule, admissionInfo, namespaceLabels, policy.GetNamespace(), gvk, subresource, policyContext.Operation()); err != nil {
		if ruleType == engineapi.Generation {
			// if the oldResource matched, return "false" to delete GR for it
			if err = engineutils.MatchesResourceDescription(oldResource, rule, admissionInfo, namespaceLabels, policy.GetNamespace(), gvk, subresource, policyContext.Operation()); err == nil {
				return engineapi.RuleFail(rule.Name, ruleType, "")
			}
		}
		logger.V(4).Info("rule not matched", "reason", err.Error())
		return nil
	}

	policyContext.JSONContext().Checkpoint()
	defer policyContext.JSONContext().Restore()

	contextLoader := e.ContextLoader(policyContext.Policy(), rule)
	if err := contextLoader(context.TODO(), rule.Context, policyContext.JSONContext()); err != nil {
		logger.V(4).Info("cannot add external data to the context", "reason", err.Error())
		return nil
	}

	ruleCopy := rule.DeepCopy()
	if after, err := variables.SubstituteAllInPreconditions(logger, ctx, ruleCopy.GetAnyAllConditions()); err != nil {
		logger.V(4).Info("failed to substitute vars in preconditions, skip current rule", "rule name", ruleCopy.Name)
		return nil
	} else {
		ruleCopy.SetAnyAllConditions(after)
	}

	// operate on the copy of the conditions, as we perform variable substitution
	copyConditions, err := utils.TransformConditions(ruleCopy.GetAnyAllConditions())
	if err != nil {
		logger.V(4).Info("cannot copy AnyAllConditions", "reason", err.Error())
		return nil
	}

	// evaluate pre-conditions
	if !variables.EvaluateConditions(logger, ctx, copyConditions) {
		logger.V(4).Info("skip rule as preconditions are not met", "rule", ruleCopy.Name)
		return engineapi.RuleSkip(ruleCopy.Name, ruleType, "")
	}

	// build rule Response
	return engineapi.RulePass(ruleCopy.Name, ruleType, "")
}
