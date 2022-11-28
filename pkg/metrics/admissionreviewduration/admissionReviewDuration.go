package admissionreviewduration

import (
	"context"
	"strings"

	"github.com/kyverno/kyverno/pkg/metrics"
	admissionv1 "k8s.io/api/admission/v1"
)

func registerAdmissionReviewDurationMetric(ctx context.Context, m *metrics.MetricsConfig, resourceKind, resourceNamespace string, resourceRequestOperation metrics.ResourceRequestOperation, admissionRequestLatency float64, allowed bool) {
	if m.Config.CheckNamespace(resourceNamespace) {
		m.RecordAdmissionReviewDuration(ctx, resourceKind, resourceNamespace, string(resourceRequestOperation), admissionRequestLatency, allowed)
	}
}

func Process(ctx context.Context, m *metrics.MetricsConfig, request *admissionv1.AdmissionRequest, response *admissionv1.AdmissionResponse, latency int64) {
	op := strings.ToLower(string(request.Operation))
	admissionReviewLatencyDurationInSeconds := float64(latency) / float64(1000*1000*1000)
	registerAdmissionReviewDurationMetric(ctx, m, request.Kind.Kind, request.Namespace, metrics.ResourceRequestOperation(op), admissionReviewLatencyDurationInSeconds, response.Allowed)
}
