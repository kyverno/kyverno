package report

import (
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	corev1 "k8s.io/api/core/v1"
)

func NewPolicyReport(namespace, name string, scope *corev1.ObjectReference, results ...policyreportv1alpha2.PolicyReportResult) kyvernov1alpha2.ReportInterface {
	var report kyvernov1alpha2.ReportInterface
	if namespace == "" {
		report = &policyreportv1alpha2.ClusterPolicyReport{
			Scope: scope,
		}
	} else {
		report = &policyreportv1alpha2.PolicyReport{
			Scope: scope,
		}
	}
	report.SetName(name)
	report.SetNamespace(namespace)
	SetManagedByKyvernoLabel(report)
	SetResults(report, results...)
	return report
}
