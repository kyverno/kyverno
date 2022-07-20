package policyexecutionduration

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/utils"
)

func registerPolicyExecutionDurationMetric(
	m *metrics.MetricsConfig,
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
	generateRuleLatencyType string,
	ruleExecutionLatency float64,
) error {
	if policyType == metrics.Cluster {
		policyNamespace = "-"
	}
	if ruleType != metrics.Generate || generateRuleLatencyType == "" {
		generateRuleLatencyType = "-"
	}
	includeNamespaces, excludeNamespaces := m.Config.GetIncludeNamespaces(), m.Config.GetExcludeNamespaces()
	if (resourceNamespace != "" && resourceNamespace != "-") && utils.ContainsString(excludeNamespaces, resourceNamespace) {
		m.Log.Info(fmt.Sprintf("Skipping the registration of kyverno_policy_execution_duration_seconds metric as the operation belongs to the namespace '%s' which is one of 'namespaces.exclude' %+v in values.yaml", resourceNamespace, excludeNamespaces))
		return nil
	}
	if (resourceNamespace != "" && resourceNamespace != "-") && len(includeNamespaces) > 0 && !utils.ContainsString(includeNamespaces, resourceNamespace) {
		m.Log.Info(fmt.Sprintf("Skipping the registration of kyverno_policy_execution_duration_seconds metric as the operation belongs to the namespace '%s' which is not one of 'namespaces.include' %+v in values.yaml", resourceNamespace, includeNamespaces))
		return nil
	}

	m.RecordPolicyExecutionDuration(policyValidationMode, policyType, policyBackgroundMode, policyNamespace, policyName, resourceKind, resourceNamespace, resourceRequestOperation, ruleName, ruleResult, ruleType, ruleExecutionCause, generateRuleLatencyType, ruleExecutionLatency)

	return nil
}

//policy - policy related data
//engineResponse - resource and rule related data
func ProcessEngineResponse(m *metrics.MetricsConfig, policy kyvernov1.PolicyInterface, engineResponse response.EngineResponse, executionCause metrics.RuleExecutionCause, generateRuleLatencyType string, resourceRequestOperation metrics.ResourceRequestOperation) error {
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
		ruleExecutionLatencyInSeconds := float64(rule.RuleStats.ProcessingTime) / float64(1000*1000*1000)
		if err := registerPolicyExecutionDurationMetric(
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
			generateRuleLatencyType,
			ruleExecutionLatencyInSeconds,
		); err != nil {
			return err
		}
	}
	return nil
}
