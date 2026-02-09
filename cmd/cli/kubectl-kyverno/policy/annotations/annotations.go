package annotations

import (
	"github.com/kyverno/kyverno/api/kyverno"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
)

func Scored(annotations map[string]string) bool {
	if scored, ok := annotations[kyverno.AnnotationPolicyScored]; ok && scored == "false" {
		return false
	}
	return true
}

func Severity(annotations map[string]string) openreportsv1alpha1.ResultSeverity {
	return reportutils.SeverityFromString(annotations[kyverno.AnnotationPolicySeverity])
}

func Category(annotations map[string]string) string {
	return annotations[kyverno.AnnotationPolicyCategory]
}
