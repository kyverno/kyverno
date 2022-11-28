package admissionrequests

import (
	"context"
	"fmt"
	"strings"

	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/utils"
	admissionv1 "k8s.io/api/admission/v1"
)

func registerAdmissionRequestsMetric(ctx context.Context, m *metrics.MetricsConfig, resourceKind, resourceNamespace string, resourceRequestOperation metrics.ResourceRequestOperation, allowed bool) {
	includeNamespaces, excludeNamespaces := m.Config.GetIncludeNamespaces(), m.Config.GetExcludeNamespaces()
	if (resourceNamespace != "" && resourceNamespace != "-") && utils.ContainsString(excludeNamespaces, resourceNamespace) {
		m.Log.V(2).Info(fmt.Sprintf("Skipping the registration of kyverno_admission_requests_total metric as the operation belongs to the namespace '%s' which is one of 'namespaces.exclude' %+v in values.yaml", resourceNamespace, excludeNamespaces))
		return
	}
	if (resourceNamespace != "" && resourceNamespace != "-") && len(includeNamespaces) > 0 && !utils.ContainsString(includeNamespaces, resourceNamespace) {
		m.Log.V(2).Info(fmt.Sprintf("Skipping the registration of kyverno_admission_requests_total metric as the operation belongs to the namespace '%s' which is not one of 'namespaces.include' %+v in values.yaml", resourceNamespace, includeNamespaces))
		return
	}
	m.RecordAdmissionRequests(ctx, resourceKind, resourceNamespace, resourceRequestOperation, allowed)
}

func Process(ctx context.Context, m *metrics.MetricsConfig, request *admissionv1.AdmissionRequest, response *admissionv1.AdmissionResponse) {
	op := strings.ToLower(string(request.Operation))
	registerAdmissionRequestsMetric(ctx, m, request.Kind.Kind, request.Namespace, metrics.ResourceRequestOperation(op), response.Allowed)
}
