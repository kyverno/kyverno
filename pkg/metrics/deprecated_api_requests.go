package metrics

import (
	"context"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func GetDeprecatedAPIRequestMetrics() DeprecatedAPIRequestMetrics {
	if metricsConfig == nil {
		return nil
	}
	return metricsConfig.DeprecatedAPIRequestMetrics()
}

type DeprecatedAPIRequestMetrics interface {
	Record(ctx context.Context, namespace, group, version, kind, field string)
}

type deprecatedAPIRequestMetrics struct {
	requestsMetric metric.Int64Counter
	logger         logr.Logger
}

func (m *deprecatedAPIRequestMetrics) init(meter metric.Meter) {
	var err error
	m.requestsMetric, err = meter.Int64Counter(
		"kyverno_deprecated_api_requests_total",
		metric.WithDescription("can be used to track requests using deprecated kyverno policy APIs and fields"),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_deprecated_api_requests_total")
	}
}

func (m *deprecatedAPIRequestMetrics) Record(ctx context.Context, namespace, group, version, kind, field string) {
	if m.requestsMetric == nil {
		return
	}
	if !GetManager().Config().CheckNamespace(namespace) {
		return
	}
	m.requestsMetric.Add(ctx, 1, metric.WithAttributes(
		attribute.String("group", group),
		attribute.String("version", version),
		attribute.String("kind", kind),
		attribute.String("field", field),
	))
}
