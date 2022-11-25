package handlers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/metrics"
	admissionRequests "github.com/kyverno/kyverno/pkg/metrics/admissionrequests"
	admissionReviewDuration "github.com/kyverno/kyverno/pkg/metrics/admissionreviewduration"
	admissionv1 "k8s.io/api/admission/v1"
)

func (inner AdmissionHandler) WithMetrics(metricsConfig *metrics.MetricsConfig) AdmissionHandler {
	return inner.withMetrics(metricsConfig).WithTrace("METRICS")
}

func (inner AdmissionHandler) withMetrics(metricsConfig *metrics.MetricsConfig) AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		response := inner(ctx, logger, request, startTime)
		defer admissionReviewDuration.Process(metricsConfig, request, response, int64(time.Since(startTime)))
		admissionRequests.Process(metricsConfig, request, response)
		return response
	}
}
