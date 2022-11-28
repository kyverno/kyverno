package admissionrequests

import (
	"context"
	"strings"

	"github.com/kyverno/kyverno/pkg/metrics"
	admissionv1 "k8s.io/api/admission/v1"
)

func registerAdmissionRequestsMetric(ctx context.Context, m *metrics.MetricsConfig, resourceKind, resourceNamespace string, resourceRequestOperation metrics.ResourceRequestOperation, allowed bool) {
	if m.Config.CheckNamespace(resourceNamespace) {
		m.RecordAdmissionRequests(ctx, resourceKind, resourceNamespace, resourceRequestOperation, allowed)
	}
}

func Process(ctx context.Context, m *metrics.MetricsConfig, request *admissionv1.AdmissionRequest, response *admissionv1.AdmissionResponse) {
	op := strings.ToLower(string(request.Operation))
	registerAdmissionRequestsMetric(ctx, m, request.Kind.Kind, request.Namespace, metrics.ResourceRequestOperation(op), response.Allowed)
}
