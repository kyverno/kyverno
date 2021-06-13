package policyruleexecutionlatency

import (
	"fmt"
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/metrics"
	prom "github.com/prometheus/client_golang/prometheus"
	"time"
)

func (pm PromMetrics) registerPolicyRuleResultsMetric(
	policyValidationMode metrics.PolicyValidationMode,
	policyType metrics.PolicyType,
	policyBackgroundMode metrics.PolicyBackgroundMode,
	policyNamespace, policyName string,
	resourceName, resourceKind, resourceNamespace string,
	resourceRequestOperation metrics.ResourceRequestOperation,
	ruleName string,
	ruleResult metrics.RuleResult,
	ruleType metrics.RuleType,
	ruleExecutionCause metrics.RuleExecutionCause,
	ruleResponse string,
	mainRequestTriggerTimestamp, policyExecutionTimestamp, ruleExecutionTimestamp int64,
	generateRuleLatencyType string,
	ruleExecutionLatencyInMs float64,
) error {
	if policyType == metrics.Cluster {
		policyNamespace = "-"
	}
	if ruleType != metrics.Generate || generateRuleLatencyType == "" {
		generateRuleLatencyType = "-"
	}
	pm.PolicyRuleExecutionLatency.With(prom.Labels{
		"policy_validation_mode":         string(policyValidationMode),
		"policy_type":                    string(policyType),
		"policy_background_mode":         string(policyBackgroundMode),
		"policy_namespace":               policyNamespace,
		"policy_name":                    policyName,
		"resource_name":                  resourceName,
		"resource_kind":                  resourceKind,
		"resource_namespace":             resourceNamespace,
		"resource_request_operation":     string(resourceRequestOperation),
		"rule_name":                      ruleName,
		"rule_result":                    string(ruleResult),
		"rule_type":                      string(ruleType),
		"rule_execution_cause":           string(ruleExecutionCause),
		"rule_response":                  ruleResponse,
		"main_request_trigger_timestamp": fmt.Sprintf("%+v", time.Unix(mainRequestTriggerTimestamp, 0)),
		"policy_execution_timestamp":     fmt.Sprintf("%+v", time.Unix(policyExecutionTimestamp, 0)),
		"rule_execution_timestamp":       fmt.Sprintf("%+v", time.Unix(ruleExecutionTimestamp, 0)),
		"generate_rule_latency_type":     generateRuleLatencyType,
	}).Set(ruleExecutionLatencyInMs)
	return nil
}

//policy - policy related data
//engineResponse - resource and rule related data
func (pm PromMetrics) ProcessEngineResponse(policy kyverno.ClusterPolicy, engineResponse response.EngineResponse, executionCause metrics.RuleExecutionCause, generateRuleLatencyType string, resourceRequestOperation metrics.ResourceRequestOperation, mainRequestTriggerTimestamp int64, logger logr.Logger) error {
	defer func() {
		if r := recover(); r != nil {
			logger.Error(fmt.Errorf("panic initiated"), "error occurred while registering kyverno_policy_rule_execution_latency_milliseconds metrics")
		}
	}()
	policyValidationMode, err := metrics.ParsePolicyValidationMode(policy.Spec.ValidationFailureAction)
	if err != nil {
		return err
	}
	policyType := metrics.Namespaced
	policyBackgroundMode := metrics.ParsePolicyBackgroundMode(*policy.Spec.Background)
	policyNamespace := policy.ObjectMeta.Namespace
	if policyNamespace == "" {
		policyNamespace = "-"
		policyType = metrics.Cluster
	}
	policyName := policy.ObjectMeta.Name

	policyExecutionTimestamp := engineResponse.PolicyResponse.PolicyExecutionTimestamp

	resourceSpec := engineResponse.PolicyResponse.Resource

	resourceName := resourceSpec.Name
	resourceKind := resourceSpec.Kind
	resourceNamespace := resourceSpec.Namespace

	ruleResponses := engineResponse.PolicyResponse.Rules

	for _, rule := range ruleResponses {
		ruleName := rule.Name
		ruleType := ParseRuleTypeFromEngineRuleResponse(rule)
		ruleResponse := rule.Message
		ruleResult := metrics.Fail
		if rule.Success {
			ruleResult = metrics.Pass
		}

		ruleExecutionTimestamp := rule.RuleStats.RuleExecutionTimestamp
		ruleExecutionLatencyInMs := float64(rule.RuleStats.ProcessingTime) / float64(1000*1000)

		if err := pm.registerPolicyRuleResultsMetric(
			policyValidationMode,
			policyType,
			policyBackgroundMode,
			policyNamespace, policyName,
			resourceName, resourceKind, resourceNamespace,
			resourceRequestOperation,
			ruleName,
			ruleResult,
			ruleType,
			executionCause,
			ruleResponse,
			mainRequestTriggerTimestamp, policyExecutionTimestamp, ruleExecutionTimestamp,
			generateRuleLatencyType,
			ruleExecutionLatencyInMs,
		); err != nil {
			return err
		}
	}
	return nil
}
