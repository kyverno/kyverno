package annotations

import (
	"github.com/kyverno/kyverno/api/kyverno"
	policyreportv1beta1 "github.com/kyverno/kyverno/api/policyreport/v1beta1"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
)

func Scored(annotations map[string]string) bool {
	if scored, ok := annotations[kyverno.AnnotationPolicyScored]; ok && scored == "false" {
		return false
	}
	return true
}

func Severity(annotations map[string]string) policyreportv1beta1.PolicyResultSeverity {
	return reportutils.SeverityFromString(annotations[kyverno.AnnotationPolicySeverity])
}

func Category(annotations map[string]string) string {
	return annotations[kyverno.AnnotationPolicyCategory]
}
