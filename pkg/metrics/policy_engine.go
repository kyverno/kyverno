package metrics

import (
	"context"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func GetPolicyEngineMetrics() PolicyEngineMetrics {
	if metricsConfig == nil {
		return nil
	}

	return metricsConfig.PolicyEngineMetrics()
}

type policyEngineMetrics struct {
	resultCounter     metric.Int64Counter
	durationHistogram metric.Float64Histogram

	logger logr.Logger
}

type PolicyEngineMetrics interface {
	RecordResponse(
		ctx context.Context,
		operation kyvernov1.AdmissionOperation,
		admissionOperation bool,
		admissionInfo kyvernov2.RequestInfo,
		response engineapi.EngineResponse,
	)
}

func (m *policyEngineMetrics) init(meter metric.Meter) {
	var err error

	m.resultCounter, err = meter.Int64Counter(
		"kyverno_policy_results",
		metric.WithDescription("can be used to track the results associated with the policies applied in the user's cluster, at the level from rule to policy to admission requests"),
	)
	if err != nil {
		m.logger.Error(err, "failed to register metric kyverno_policy_results")
	}
	m.durationHistogram, err = meter.Float64Histogram(
		"kyverno_policy_execution_duration_seconds",
		metric.WithDescription("can be used to track the latencies (in seconds) associated with the execution/processing of the individual rules under Kyverno policies whenever they evaluate incoming resource requests"),
	)
	if err != nil {
		m.logger.Error(err, "failed to register metric kyverno_policy_execution_duration_seconds")
	}
}

func (m *policyEngineMetrics) RecordResult(ctx context.Context, policyName string) {
	m.resultCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("policy_name", policyName)))
}

func (m *policyEngineMetrics) RecordResponse(
	ctx context.Context,
	operation kyvernov1.AdmissionOperation,
	admissionOperation bool,
	admissionInfo kyvernov2.RequestInfo,
	response engineapi.EngineResponse,
) {
	if m.resultCounter == nil || m.durationHistogram == nil {
		return
	}

	policy := response.Policy().AsKyvernoPolicy()

	name, namespace, policyType, backgroundMode, validationMode, err := GetPolicyInfos(policy)
	if err != nil {
		m.logger.Error(err, "failed to get policy infos for metrics reporting")
	}

	if policyType == Cluster {
		namespace = "-"
	}
	if !GetManager().Config().CheckNamespace(namespace) {
		return
	}

	resourceSpec := response.Resource
	resourceKind := resourceSpec.GetKind()
	resourceNamespace := resourceSpec.GetNamespace()
	for _, rule := range response.PolicyResponse.Rules {
		ruleName := rule.Name()
		ruleType := ParseRuleTypeFromEngineRuleResponse(rule)

		var ruleResult RuleResult
		switch rule.Status() {
		case engineapi.RuleStatusPass:
			ruleResult = Pass
		case engineapi.RuleStatusFail:
			ruleResult = Fail
		case engineapi.RuleStatusWarn:
			ruleResult = Warn
		case engineapi.RuleStatusError:
			ruleResult = Error
		case engineapi.RuleStatusSkip:
			ruleResult = Skip
		default:
			ruleResult = Fail
		}

		executionCause := AdmissionRequest
		if !admissionOperation {
			executionCause = BackgroundScan
		}

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
			attribute.String("dry_run", strconv.FormatBool(admissionInfo.DryRun)),
		}

		m.resultCounter.Add(ctx, 1, metric.WithAttributes(commonLabels...))
		m.durationHistogram.Record(ctx, rule.Stats().ProcessingTime().Seconds(), metric.WithAttributes(commonLabels...))
	}
}
