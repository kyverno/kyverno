package policyexecutionduration

import (
	"fmt"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/metrics"
	prom "github.com/prometheus/client_golang/prometheus"
)

func (pc PromConfig) registerPolicyExecutionDurationMetric(
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
	includeNamespaces, excludeNamespaces := pc.Config.GetIncludeNamespaces(), pc.Config.GetExcludeNamespaces()
	if (resourceNamespace != "" && resourceNamespace != "-") && metrics.ElementInSlice(resourceNamespace, excludeNamespaces) {
		pc.Log.Info(fmt.Sprintf("Skipping the registration of kyverno_policy_execution_duration_seconds metric as the operation belongs to the namespace '%s' which is one of 'namespaces.exclude' %+v in values.yaml", resourceNamespace, excludeNamespaces))
		return nil
	}
	if (resourceNamespace != "" && resourceNamespace != "-") && len(includeNamespaces) > 0 && !metrics.ElementInSlice(resourceNamespace, includeNamespaces) {
		pc.Log.Info(fmt.Sprintf("Skipping the registration of kyverno_policy_execution_duration_seconds metric as the operation belongs to the namespace '%s' which is not one of 'namespaces.include' %+v in values.yaml", resourceNamespace, includeNamespaces))
		return nil
	}
	pc.Metrics.PolicyExecutionDuration.With(prom.Labels{
		"policy_validation_mode":     string(policyValidationMode),
		"policy_type":                string(policyType),
		"policy_background_mode":     string(policyBackgroundMode),
		"policy_namespace":           policyNamespace,
		"policy_name":                policyName,
		"resource_kind":              resourceKind,
		"resource_namespace":         resourceNamespace,
		"resource_request_operation": string(resourceRequestOperation),
		"rule_name":                  ruleName,
		"rule_result":                string(ruleResult),
		"rule_type":                  string(ruleType),
		"rule_execution_cause":       string(ruleExecutionCause),
		"generate_rule_latency_type": generateRuleLatencyType,
	}).Observe(ruleExecutionLatency)
	return nil
}

//policy - policy related data
//engineResponse - resource and rule related data
func (pc PromConfig) ProcessEngineResponse(policy kyverno.ClusterPolicy, engineResponse response.EngineResponse, executionCause metrics.RuleExecutionCause, generateRuleLatencyType string, resourceRequestOperation metrics.ResourceRequestOperation) error {

	policyValidationMode, err := metrics.ParsePolicyValidationMode(policy.Spec.ValidationFailureAction)
	if err != nil {
		return err
	}
	policyType := metrics.Namespaced
	policyBackgroundMode := metrics.ParsePolicyBackgroundMode(policy.Spec.Background)
	policyNamespace := policy.ObjectMeta.Namespace
	if policyNamespace == "" {
		policyNamespace = "-"
		policyType = metrics.Cluster
	}
	policyName := policy.ObjectMeta.Name

	resourceSpec := engineResponse.PolicyResponse.Resource

	resourceKind := resourceSpec.Kind
	resourceNamespace := resourceSpec.Namespace

	ruleResponses := engineResponse.PolicyResponse.Rules

	for _, rule := range ruleResponses {
		ruleName := rule.Name
		ruleType := ParseRuleTypeFromEngineRuleResponse(rule)

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

		if err := pc.registerPolicyExecutionDurationMetric(
			policyValidationMode,
			policyType,
			policyBackgroundMode,
			policyNamespace, policyName,
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
