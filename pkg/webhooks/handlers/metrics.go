package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func (inner AdmissionHandler) WithMetrics(logger logr.Logger, metricsConfig config.MetricsConfiguration, attrs ...attribute.KeyValue) AdmissionHandler {
	return inner.withMetrics(logger, metricsConfig, attrs...).WithTrace("METRICS")
}

func (inner AdmissionHandler) withMetrics(logger logr.Logger, metricsConfig config.MetricsConfiguration, attrs ...attribute.KeyValue) AdmissionHandler {
	meter := global.MeterProvider().Meter(metrics.MeterName)
	requestsMetric, err := meter.Int64Counter(
		"kyverno_admission_requests",
		instrument.WithDescription("can be used to track the number of admission requests encountered by Kyverno in the cluster"),
	)
	if err != nil {
		logger.Error(err, "Failed to create instrument, kyverno_admission_requests_total")
	}
	durationMetric, err := meter.Float64Histogram(
		"kyverno_admission_review_duration_seconds",
		instrument.WithDescription("can be used to track the latencies (in seconds) associated with the entire individual admission review. For example, if an incoming request trigger, say, five policies, this metric will track the e2e latency associated with the execution of all those policies"),
	)
	if err != nil {
		logger.Error(err, "Failed to create instrument, kyverno_admission_review_duration_seconds")
	}
	return func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		response := inner(ctx, logger, request, startTime)
		namespace := request.Namespace
		if metricsConfig.CheckNamespace(namespace) {
			operation := strings.ToLower(string(request.Operation))
			attributes := []attribute.KeyValue{
				attribute.String("resource_kind", request.Kind.Kind),
				attribute.String("resource_namespace", namespace),
				attribute.String("resource_request_operation", operation),
				attribute.Bool("request_allowed", response.Allowed),
			}
			attributes = append(attributes, attrs...)
			if durationMetric != nil {
				defer func() {
					latency := int64(time.Since(startTime))
					durationInSeconds := float64(latency) / float64(1000*1000*1000)
					durationMetric.Record(ctx, durationInSeconds, attributes...)
				}()
			}
			if requestsMetric != nil {
				requestsMetric.Add(ctx, 1, attributes...)
			}
		}
		return response
	}
}

func (inner HttpHandler) WithMetrics(logger logr.Logger, attrs ...attribute.KeyValue) HttpHandler {
	return inner.withMetrics(logger, attrs...).WithTrace("METRICS")
}

func (inner HttpHandler) withMetrics(logger logr.Logger, attrs ...attribute.KeyValue) HttpHandler {
	meter := global.MeterProvider().Meter(metrics.MeterName)
	requestsMetric, err := meter.Int64Counter(
		"kyverno_http_requests",
		instrument.WithDescription("can be used to track the number of http requests"),
	)
	if err != nil {
		logger.Error(err, "Failed to create instrument, kyverno_http_requests")
	}
	durationMetric, err := meter.Float64Histogram(
		"kyverno_http_requests_duration_seconds",
		instrument.WithDescription("can be used to track the latencies (in seconds) associated with the entire individual http request."),
	)
	if err != nil {
		logger.Error(err, "Failed to create instrument, kyverno_http_requests_duration_seconds")
	}
	return func(writer http.ResponseWriter, request *http.Request) {
		startTime := time.Now()
		attributes := []attribute.KeyValue{
			// semconv.HTTPHostKey.String(request.Host),
			semconv.HTTPMethodKey.String(request.Method),
			semconv.HTTPURLKey.String(request.RequestURI),
		}
		attributes = append(attributes, attrs...)
		if requestsMetric != nil {
			requestsMetric.Add(request.Context(), 1, attributes...)
		}
		if durationMetric != nil {
			defer func() {
				latency := int64(time.Since(startTime))
				durationInSeconds := float64(latency) / float64(1000*1000*1000)
				durationMetric.Record(request.Context(), durationInSeconds, attributes...)
			}()
		}
		inner(writer, request)
	}
}
