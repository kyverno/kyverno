package policyexecutionduration

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/metrics"
)

func registerPolicyExecutionDurationMetric(
	ctx context.Context,
	m metrics.MetricsConfigManager,
	policyValidationMode metrics.PolicyValidationMode,
	policyType metrics.PolicyType,
	policyBackgroundMode metrics.PolicyBackgroundMode,
	policyNamespace, policyName string,
	resourceNamespace string,
	ruleName string,
	ruleResult metrics.RuleResult,
	ruleType metrics.RuleType,
	ruleExecutionCause metrics.RuleExecutionCause,
	ruleExecutionLatency float64,
) {
	if policyType == metrics.Cluster {
		policyNamespace = "-"
	}
	if m.Config().CheckNamespace(policyNamespace) {
		m.RecordPolicyExecutionDuration(ctx, policyValidationMode, policyType, policyBackgroundMode, policyNamespace, policyName, ruleName, ruleResult, ruleType, ruleExecutionCause, ruleExecutionLatency)
	}
}

// policy - policy related data
// engineResponse - resource and rule related data
func ProcessEngineResponse(ctx context.Context, m metrics.MetricsConfigManager, policy kyvernov1.PolicyInterface, engineResponse engineapi.EngineResponse, executionCause metrics.RuleExecutionCause, resourceRequestOperation metrics.ResourceRequestOperation) error {
	name, namespace, policyType, backgroundMode, validationMode, err := metrics.GetPolicyInfos(policy)
	if err != nil {
		return err
	}
	resourceSpec := engineResponse.Resource
	resourceNamespace := resourceSpec.GetNamespace()
	ruleResponses := engineResponse.PolicyResponse.Rules
	for _, rule := range ruleResponses {
		ruleName := rule.Name
		ruleType := metrics.ParseRuleTypeFromEngineRuleResponse(rule)
		var ruleResult metrics.RuleResult
		switch rule.Status {
		case engineapi.RuleStatusPass:
			ruleResult = metrics.Pass
		case engineapi.RuleStatusFail:
			ruleResult = metrics.Fail
		case engineapi.RuleStatusWarn:
			ruleResult = metrics.Warn
		case engineapi.RuleStatusError:
			ruleResult = metrics.Error
		case engineapi.RuleStatusSkip:
			ruleResult = metrics.Skip
		default:
			ruleResult = metrics.Fail
		}
		ruleExecutionLatencyInSeconds := float64(rule.Stats.ProcessingTime) / float64(1000*1000*1000)
		registerPolicyExecutionDurationMetric(
			ctx,
			m,
			validationMode,
			policyType,
			backgroundMode,
			namespace, name,
			resourceNamespace,
			ruleName,
			ruleResult,
			ruleType,
			executionCause,
			ruleExecutionLatencyInSeconds,
		)
	}
	return nil
}
