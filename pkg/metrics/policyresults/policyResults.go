package policyresults

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/metrics"
)

func registerPolicyResultsMetric(
	ctx context.Context,
	m metrics.MetricsConfigManager,
	policyValidationMode metrics.PolicyValidationMode,
	policyType metrics.PolicyType,
	policyBackgroundMode metrics.PolicyBackgroundMode,
	policyNamespace, policyName string,
	resourceKind, resourceNamespace string,
	resourceRequestOperation metrics.ResourceRequestOperation,
	ruleName string,
	ruleResult metrics.RuleResult,
	ruleType metrics.RuleType,
	ruleExecutionCause metrics.RuleExecutionCause,
) {
	if policyType == metrics.Cluster {
		policyNamespace = "-"
	}
	if m.Config().CheckNamespace(policyNamespace) {
		m.RecordPolicyResults(ctx, policyValidationMode, policyType, policyBackgroundMode, policyNamespace, policyName, resourceKind, resourceNamespace, resourceRequestOperation, ruleName, ruleResult, ruleType, ruleExecutionCause)
	}
}

// policy - policy related data
// engineResponse - resource and rule related data
func ProcessEngineResponse(ctx context.Context, m metrics.MetricsConfigManager, policy kyvernov1.PolicyInterface, engineResponse response.EngineResponse, executionCause metrics.RuleExecutionCause, resourceRequestOperation metrics.ResourceRequestOperation) error {
	name, namespace, policyType, backgroundMode, validationMode, err := metrics.GetPolicyInfos(policy)
	if err != nil {
		return err
	}
	resourceSpec := engineResponse.PolicyResponse.Resource
	resourceKind := resourceSpec.Kind
	resourceNamespace := resourceSpec.Namespace
	ruleResponses := engineResponse.PolicyResponse.Rules
	for _, rule := range ruleResponses {
		ruleName := rule.Name
		ruleType := metrics.ParseRuleTypeFromEngineRuleResponse(rule)
		var ruleResult metrics.RuleResult
		switch rule.Status {
		case response.RuleStatusPass:
			ruleResult = metrics.Pass
		case response.RuleStatusFail:
			ruleResult = metrics.Fail
		case response.RuleStatusWarn:
			ruleResult = metrics.Warn
		case response.RuleStatusError:
			ruleResult = metrics.Error
		case response.RuleStatusSkip:
			ruleResult = metrics.Skip
		default:
			ruleResult = metrics.Fail
		}
		registerPolicyResultsMetric(
			ctx,
			m,
			validationMode,
			policyType,
			backgroundMode,
			namespace, name,
			resourceKind, resourceNamespace,
			resourceRequestOperation,
			ruleName,
			ruleResult,
			ruleType,
			executionCause,
		)
	}
	return nil
}
