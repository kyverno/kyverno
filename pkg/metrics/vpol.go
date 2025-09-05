package metrics

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
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
	RecordDuration(ctx context.Context, seconds float64, status, ruleExecutionCause string, policy v1alpha1.ValidatingPolicy, resource *unstructured.Unstructured, operation string)
}

type validatingMetrics struct {
	durationHistogram metric.Float64Histogram

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
}

func (m *validatingMetrics) RecordDuration(ctx context.Context, seconds float64, status, ruleExecutionCause string, policy v1alpha1.ValidatingPolicy, resource *unstructured.Unstructured, operation string) {
	if m.durationHistogram == nil {
		return
	}

	name, _, backgroundMode, validationMode := GetCELPolicyInfos(&policy)

	m.durationHistogram.Record(ctx, seconds, metric.WithAttributes(
		attribute.String("policy_validation_mode", string(validationMode)),
		attribute.String("policy_background_mode", string(backgroundMode)),
		attribute.String("policy_name", name),
		attribute.String("resource_kind", resource.GetKind()),
		attribute.String("resource_namespace", resource.GetNamespace()),
		attribute.String("resource_request_operation", strings.ToLower(operation)),
		attribute.String("rule_execution_cause", ruleExecutionCause),
		attribute.String("result", status),
	))
}
