package handlers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/metrics"
	admissionRequests "github.com/kyverno/kyverno/pkg/metrics/admissionrequests"
	admissionReviewDuration "github.com/kyverno/kyverno/pkg/metrics/admissionreviewduration"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/trace"
	admissionv1 "k8s.io/api/admission/v1"
)

func (h AdmissionHandler) WithMetrics(metricsConfig *metrics.MetricsConfig) AdmissionHandler {
	return withMetrics(metricsConfig, h)
}

func withMetrics(metricsConfig *metrics.MetricsConfig, inner AdmissionHandler) AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		return tracing.Span1(
			ctx,
			"admission_webhook_operations",
			"dump",
			func(ctx context.Context, span trace.Span) *admissionv1.AdmissionResponse {
				defer admissionReviewDuration.Process(metricsConfig, request, int64(time.Since(startTime)))
				admissionRequests.Process(metricsConfig, request)
				return inner(ctx, logger, request, startTime)
			},
		)
	}
}
