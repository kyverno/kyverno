package mutation

import (
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func (v *mutationHandler) needsReports(request handlers.AdmissionRequest, admissionReport bool) bool {
	createReport := admissionReport
	if admissionutils.IsDryRun(request.AdmissionRequest) {
		createReport = false
	}
	if !v.reportsConfig.MutateReportsEnabled() {
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

	return createReport
}
