package admissionreviewduration

import (
	"fmt"
	"strings"

	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/utils"
	admissionv1 "k8s.io/api/admission/v1"
)

func registerAdmissionReviewDurationMetric(m *metrics.MetricsConfig, resourceKind, resourceNamespace string, resourceRequestOperation metrics.ResourceRequestOperation, admissionRequestLatency float64) {
	includeNamespaces, excludeNamespaces := m.Config.GetIncludeNamespaces(), m.Config.GetExcludeNamespaces()
	if (resourceNamespace != "" && resourceNamespace != "-") && utils.ContainsString(excludeNamespaces, resourceNamespace) {
		m.Log.V(2).Info(fmt.Sprintf("Skipping the registration of kyverno_admission_review_duration_seconds metric as the operation belongs to the namespace '%s' which is one of 'namespaces.exclude' %+v in values.yaml", resourceNamespace, excludeNamespaces))
		return
	}
	if (resourceNamespace != "" && resourceNamespace != "-") && len(includeNamespaces) > 0 && !utils.ContainsString(includeNamespaces, resourceNamespace) {
		m.Log.V(2).Info(fmt.Sprintf("Skipping the registration of kyverno_admission_review_duration_seconds metric as the operation belongs to the namespace '%s' which is not one of 'namespaces.include' %+v in values.yaml", resourceNamespace, includeNamespaces))
		return
	}
	m.RecordAdmissionReviewDuration(resourceKind, resourceNamespace, string(resourceRequestOperation), admissionRequestLatency)
}

func Process(m *metrics.MetricsConfig, request *admissionv1.AdmissionRequest, latency int64) {
	op := strings.ToLower(string(request.Operation))
	admissionReviewLatencyDurationInSeconds := float64(latency) / float64(1000*1000*1000)
	registerAdmissionReviewDurationMetric(m, request.Kind.Kind, request.Namespace, metrics.ResourceRequestOperation(op), admissionReviewLatencyDurationInSeconds)
}
