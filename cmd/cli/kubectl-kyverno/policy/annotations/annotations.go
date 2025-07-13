package annotations

import (
	"github.com/kyverno/kyverno/api/kyverno"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
)

func Scored(annotations map[string]string) bool {
	if scored, ok := annotations[kyverno.AnnotationPolicyScored]; ok && scored == "false" {
		return false
	}
	return true
}

func Severity(annotations map[string]string) policyreportv1alpha2.PolicySeverity {
	return reportutils.SeverityFromString(annotations[kyverno.AnnotationPolicySeverity])
}

func Category(annotations map[string]string) string {
	return annotations[kyverno.AnnotationPolicyCategory]
}
