package engine

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func (e *engine) reportMetrics(
	ctx context.Context,
	logger logr.Logger,
	operation kyvernov1.AdmissionOperation,
	admissionOperation bool,
	response engineapi.EngineResponse,
) {
	if e.resultCounter == nil && e.durationHistogram == nil {
		return
	}
	policy := response.Policy().GetPolicy().(kyvernov1.PolicyInterface)
	if name, namespace, policyType, backgroundMode, validationMode, err := metrics.GetPolicyInfos(policy); err != nil {
		logger.Error(err, "failed to get policy infos for metrics reporting")
	} else {
		if policyType == metrics.Cluster {
			namespace = "-"
		}
		if !e.metricsConfiguration.CheckNamespace(namespace) {
			return
		}
		resourceSpec := response.Resource
		resourceKind := resourceSpec.GetKind()
		resourceNamespace := resourceSpec.GetNamespace()
		for _, rule := range response.PolicyResponse.Rules {
			ruleName := rule.Name()
			ruleType := metrics.ParseRuleTypeFromEngineRuleResponse(rule)
			var ruleResult metrics.RuleResult
			switch rule.Status() {
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
			executionCause := metrics.AdmissionRequest
			if !admissionOperation {
				executionCause = metrics.BackgroundScan
			}
			if e.resultCounter != nil {
				commonLabels := []attribute.KeyValue{
					attribute.String("policy_validation_mode", string(validationMode)),
					attribute.String("policy_type", string(policyType)),
					attribute.String("policy_background_mode", string(backgroundMode)),
					attribute.String("policy_namespace", namespace),
					attribute.String("policy_name", name),
					attribute.String("resource_kind", resourceKind),
					attribute.String("resource_namespace", resourceNamespace),
					attribute.String("resource_request_operation", strings.ToLower(string(operation))),
					attribute.String("rule_name", ruleName),
					attribute.String("rule_result", string(ruleResult)),
					attribute.String("rule_type", string(ruleType)),
					attribute.String("rule_execution_cause", string(executionCause)),
				}
				e.resultCounter.Add(ctx, 1, metric.WithAttributes(commonLabels...))
			}
			if e.durationHistogram != nil {
				commonLabels := []attribute.KeyValue{
					attribute.String("policy_validation_mode", string(validationMode)),
					attribute.String("policy_type", string(policyType)),
					attribute.String("policy_background_mode", string(backgroundMode)),
					attribute.String("policy_namespace", namespace),
					attribute.String("policy_name", name),
					attribute.String("resource_kind", resourceKind),
					attribute.String("resource_namespace", resourceNamespace),
					attribute.String("resource_request_operation", strings.ToLower(string(operation))),
					attribute.String("rule_name", ruleName),
					attribute.String("rule_result", string(ruleResult)),
					attribute.String("rule_type", string(ruleType)),
					attribute.String("rule_execution_cause", string(executionCause)),
				}
				e.durationHistogram.Record(ctx, rule.Stats().ProcessingTime().Seconds(), metric.WithAttributes(commonLabels...))
			}
		}
	}
}
