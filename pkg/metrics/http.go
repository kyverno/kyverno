package metrics

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func GetHTTPMetrics() HTTPMetrics {
	if metricsConfig == nil {
		return nil
	}

	return metricsConfig.HTTPMetrics()
}

type httpMetrics struct {
	requestsMetric metric.Int64Counter
	durationMetric metric.Float64Histogram

	logger logr.Logger
}

type HTTPMetrics interface {
	RecordRequest(ctx context.Context, method string, uri string, startTime time.Time, attrs ...attribute.KeyValue)
}

func (m *httpMetrics) init(meterProvider metric.MeterProvider) {
	var err error
	meter := meterProvider.Meter(MeterName)

	m.requestsMetric, err = meter.Int64Counter(
		"kyverno_http_requests",
		metric.WithDescription("can be used to track the number of http requests"),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_http_requests")
	}
	m.durationMetric, err = meter.Float64Histogram(
		"kyverno_http_requests_duration_seconds",
		metric.WithDescription("can be used to track the latencies (in seconds) associated with the entire individual http request."),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_http_requests_duration_seconds")
	}
}

func (m *httpMetrics) RecordRequest(ctx context.Context, method string, uri string, startTime time.Time, attrs ...attribute.KeyValue) {
	attributes := append([]attribute.KeyValue{
		semconv.HTTPMethodKey.String(method),
		semconv.HTTPURLKey.String(uri),
	}, attrs...)

	m.requestsMetric.Add(ctx, 1, metric.WithAttributes(attributes...))

	if m.durationMetric != nil {
		defer func() {
			latency := int64(time.Since(startTime))
			m.durationMetric.Record(ctx, float64(latency)/float64(1000*1000*1000), metric.WithAttributes(attributes...))
		}()
	}
}
