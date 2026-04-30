package metrics

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetValidatingMetrics() ValidatingMetrics {
	if metricsConfig == nil {
		return nil
	}

	return metricsConfig.VPOLMetrics()
}

type ValidatingMetrics interface {
	RecordDuration(ctx context.Context, seconds float64, status, ruleExecutionCause string, policy v1beta1.ValidatingPolicyLike, resource *unstructured.Unstructured, operation string)
	RecordResult(ctx context.Context, status, ruleExecutionCause string, policy v1beta1.ValidatingPolicyLike, resource *unstructured.Unstructured, operation string)
}

type validatingMetrics struct {
	durationHistogram metric.Float64Histogram
	resultCounter     metric.Int64Counter

	logger logr.Logger
}

func (m *validatingMetrics) init(meter metric.Meter) {
	var err error

	m.durationHistogram, err = meter.Float64Histogram(
		"kyverno_validating_policy_execution_duration_seconds",
		metric.WithDescription("can be used to track the latencies (in seconds) associated with the execution/processing of individual validation policies when they evaluate incoming resource requests."),
	)
	if err != nil {
		m.logger.Error(err, "failed to register metric kyverno_validating_policy_execution_duration_seconds")
	}

	m.resultCounter, err = meter.Int64Counter(
		"kyverno_validating_policy_results",
		metric.WithDescription("can be used to track the results associated with the validating policies applied in the user's cluster, at the level from rule to policy to admission requests."),
	)
	if err != nil {
		m.logger.Error(err, "failed to register metric kyverno_validating_policy_results")
	}
}

func (m *validatingMetrics) RecordDuration(ctx context.Context, seconds float64, status, ruleExecutionCause string, policy v1beta1.ValidatingPolicyLike, resource *unstructured.Unstructured, operation string) {
	if m.durationHistogram == nil {
		return
	}

	name := policy.GetName()
	backgroundMode := policy.BackgroundEnabled()
	validationMode := policy.GetValidatingPolicySpec().EvaluationMode()

	m.durationHistogram.Record(ctx, seconds, metric.WithAttributes(
		attribute.String("policy_validation_mode", validationMode),
		attribute.String("policy_background_mode", fmt.Sprintf("%t", backgroundMode)),
		attribute.String("policy_name", name),
		attribute.String("resource_kind", resource.GetKind()),
		attribute.String("resource_namespace", resource.GetNamespace()),
		attribute.String("resource_request_operation", strings.ToLower(operation)),
		attribute.String("execution_cause", ruleExecutionCause),
		attribute.String("result", status),
	))
}

func (m *validatingMetrics) RecordResult(ctx context.Context, status, ruleExecutionCause string, policy v1beta1.ValidatingPolicyLike, resource *unstructured.Unstructured, operation string) {
	if m.resultCounter == nil {
		return
	}

	name := policy.GetName()
	backgroundMode := policy.BackgroundEnabled()
	validationMode := policy.GetValidatingPolicySpec().EvaluationMode()

	m.resultCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("policy_validation_mode", validationMode),
		attribute.String("policy_background_mode", fmt.Sprintf("%t", backgroundMode)),
		attribute.String("policy_name", name),
		attribute.String("resource_kind", resource.GetKind()),
		attribute.String("resource_namespace", resource.GetNamespace()),
		attribute.String("resource_request_operation", strings.ToLower(operation)),
		attribute.String("execution_cause", ruleExecutionCause),
		attribute.String("result", status),
	))
}
