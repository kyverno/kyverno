package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	admissionv1 "k8s.io/api/admission/v1"
)

func (inner AdmissionHandler) WithMetrics(logger logr.Logger, metricsConfig config.MetricsConfiguration) AdmissionHandler {
	return inner.withMetrics(logger, metricsConfig).WithTrace("METRICS")
}

func (inner AdmissionHandler) withMetrics(logger logr.Logger, metricsConfig config.MetricsConfiguration) AdmissionHandler {
	meter := global.MeterProvider().Meter("kyverno")
	admissionRequestsMetric, err := meter.SyncInt64().Counter(
		"kyverno_admission_requests_total",
		instrument.WithDescription("can be used to track the number of admission requests encountered by Kyverno in the cluster"),
	)
	if err != nil {
		logger.Error(err, "Failed to create instrument, kyverno_admission_requests_total")
	}
	admissionReviewDurationMetric, err := meter.SyncFloat64().Histogram(
		"kyverno_admission_review_duration_seconds",
		instrument.WithDescription("can be used to track the latencies (in seconds) associated with the entire individual admission review. For example, if an incoming request trigger, say, five policies, this metric will track the e2e latency associated with the execution of all those policies"),
	)
	if err != nil {
		logger.Error(err, "Failed to create instrument, kyverno_admission_review_duration_seconds")
	}
	return func(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		response := inner(ctx, logger, request, startTime)
		namespace := request.Namespace
		if metricsConfig.CheckNamespace(namespace) {
			operation := strings.ToLower(string(request.Operation))
			allowed := true
			if response != nil {
				allowed = response.Allowed
			}
			if admissionReviewDurationMetric != nil {
				defer func() {
					latency := int64(time.Since(startTime))
					admissionReviewLatencyDurationInSeconds := float64(latency) / float64(1000*1000*1000)
					admissionReviewDurationMetric.Record(
						ctx,
						admissionReviewLatencyDurationInSeconds,
						attribute.String("resource_kind", request.Kind.Kind),
						attribute.String("resource_namespace", namespace),
						attribute.String("resource_request_operation", operation),
						attribute.Bool("request_allowed", allowed),
					)
				}()
			}
			if admissionRequestsMetric != nil {
				admissionRequestsMetric.Add(
					ctx,
					1,
					attribute.String("resource_kind", request.Kind.Kind),
					attribute.String("resource_namespace", namespace),
					attribute.String("resource_request_operation", operation),
					attribute.Bool("request_allowed", allowed),
				)
			}
		}
		return response
	}
}
