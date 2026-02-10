package metrics

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetGeneratingMetrics() GeneratingMetrics {
	if metricsConfig == nil {
		return nil
	}

	return metricsConfig.GPOLMetrics()
}

type GeneratingMetrics interface {
	RecordDuration(ctx context.Context, seconds float64, status string, policy v1beta1.GeneratingPolicyLike, resource *unstructured.Unstructured, operation string)
}

type generatingMetrics struct {
	durationHistogram metric.Float64Histogram

	logger logr.Logger
}

func (m *generatingMetrics) init(meter metric.Meter) {
	var err error

	m.durationHistogram, err = meter.Float64Histogram(
		"kyverno_generating_policy_execution_duration_seconds",
		metric.WithDescription("can be used to track the latencies (in seconds) associated with the execution/processing of individual generating policies when they evaluate incoming resource requests."),
	)
	if err != nil {
		m.logger.Error(err, "failed to register metric kyverno_generating_policy_execution_duration_seconds")
	}
}

func (m *generatingMetrics) RecordDuration(ctx context.Context, seconds float64, status string, policy v1beta1.GeneratingPolicyLike, resource *unstructured.Unstructured, operation string) {
	if m.durationHistogram == nil {
		return
	}

	m.durationHistogram.Record(ctx, seconds, metric.WithAttributes(
		attribute.String("policy_name", policy.GetName()),
		attribute.String("resource_kind", resource.GetKind()),
		attribute.String("resource_namespace", resource.GetNamespace()),
		attribute.String("resource_request_operation", strings.ToLower(operation)),
		attribute.String("result", status),
	))
}
