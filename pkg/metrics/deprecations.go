package metrics

import (
	"context"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func GetDeprecationMetrics() DeprecationMetrics {
	if metricsConfig == nil {
		return nil
	}

	return metricsConfig.DeprecationMetrics()
}

type deprecationsMetrics struct {
	requestsMetric metric.Int64Counter

	logger logr.Logger
}

type DeprecationMetrics interface {
	RecordDeprecatedAPIRequest(ctx context.Context, group, version, kind, field string)
}

func (m *deprecationsMetrics) init(meter metric.Meter) {
	var err error

	m.requestsMetric, err = meter.Int64Counter(
		"kyverno_deprecated_api_requests_total",
		metric.WithDescription("can be used to track the number of requests that use a deprecated Kyverno API version or field"),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_deprecated_api_requests_total")
	}
}

func (m *deprecationsMetrics) RecordDeprecatedAPIRequest(ctx context.Context, group, version, kind, field string) {
	if m.requestsMetric == nil {
		return
	}

	attributes := []attribute.KeyValue{
		attribute.String("group", group),
		attribute.String("version", version),
		attribute.String("kind", kind),
		attribute.String("field", field),
	}

	m.requestsMetric.Add(ctx, 1, metric.WithAttributes(attributes...))
}
