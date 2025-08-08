package validation

import (
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func NeedsReports(request handlers.AdmissionRequest, resource unstructured.Unstructured, admissionReport bool, reportConfig reportutils.ReportingConfiguration) bool {
	createReport := admissionReport
	if admissionutils.IsDryRun(request.AdmissionRequest) {
		createReport = false
	}
	if !reportConfig.ValidateReportsEnabled() {
		createReport = false
	}
	// we don't need reports for deletions
	if request.Operation == admissionv1.Delete {
		createReport = false
	}
	// check if the resource supports reporting
	if !reportutils.IsGvkSupported(schema.GroupVersionKind(request.Kind)) {
		createReport = false
	}
	// if the underlying resource has no UID don't create a report
	if resource.GetUID() == "" {
		createReport = false
	}
	return createReport
}
