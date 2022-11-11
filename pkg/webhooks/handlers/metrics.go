package handlers

import (
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/metrics"
	admissionRequests "github.com/kyverno/kyverno/pkg/metrics/admissionrequests"
	admissionReviewDuration "github.com/kyverno/kyverno/pkg/metrics/admissionreviewduration"
	admissionv1 "k8s.io/api/admission/v1"
)

func (h AdmissionHandler) WithMetrics(metricsConfig *metrics.MetricsConfig) AdmissionHandler {
	return withMetrics(metricsConfig, h)
}

func withMetrics(metricsConfig *metrics.MetricsConfig, inner AdmissionHandler) AdmissionHandler {
	return func(logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		defer admissionReviewDuration.Process(metricsConfig, request, int64(time.Since(startTime)))
		admissionRequests.Process(metricsConfig, request)
		return inner(logger, request, startTime)
	}
}
