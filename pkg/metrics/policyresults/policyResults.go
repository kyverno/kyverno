package policyresults

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
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
func ProcessEngineResponse(ctx context.Context, m metrics.MetricsConfigManager, policy kyvernov1.PolicyInterface, engineResponse engineapi.EngineResponse, executionCause metrics.RuleExecutionCause, resourceRequestOperation metrics.ResourceRequestOperation) error {
	name, namespace, policyType, backgroundMode, validationMode, err := metrics.GetPolicyInfos(policy)
	if err != nil {
		return err
	}
	resourceSpec := engineResponse.Resource
	resourceKind := resourceSpec.GetKind()
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

func ProcessCleanupResponse(ctx context.Context, m metrics.MetricsConfigManager, policy kyvernov2alpha1.CleanupPolicyInterface, cleanupResponse engineapi.CleanupResponse, resourceRequestOperation metrics.ResourceRequestOperation) error {
	name := policy.GetName()
	namespace := ""
	policyType := metrics.Cluster
	if policy.IsNamespaced() {
		namespace = policy.GetNamespace()
		policyType = metrics.Namespaced
	}
	policyResponse := cleanupResponse.PolicyResponse
	resourceSpec := policyResponse.Resource
	resourceKind := resourceSpec.GetKind()
	resourceNamespace := resourceSpec.GetNamespace()

	// TODO
	var cleanupResult metrics.CleanupResult = ""
	registerCleanupPolicyResultsMetric(
		ctx,
		m,
		policyType,
		namespace, name,
		resourceKind, resourceNamespace,
		resourceRequestOperation,
		cleanupResult,
	)
	return nil
}

func registerCleanupPolicyResultsMetric(
	ctx context.Context,
	m metrics.MetricsConfigManager,
	policyType metrics.PolicyType,
	policyNamespace, policyName string,
	resourceKind, resourceNamespace string,
	resourceRequestOperation metrics.ResourceRequestOperation,
	cleanupResult metrics.CleanupResult,
) {
	if policyType == metrics.Cluster {
		policyNamespace = "-"
	}
	if m.Config().CheckNamespace(policyNamespace) {
		m.RecordCleanupResults(ctx, policyType, policyNamespace, policyName, resourceKind, resourceNamespace, resourceRequestOperation, cleanupResult)
	}
}
