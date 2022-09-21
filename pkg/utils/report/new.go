package report

import (
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
)

func NewReport(namespace, name string) kyvernov1alpha2.ReportChangeRequestInterface {
	var report kyvernov1alpha2.ReportChangeRequestInterface
	if namespace == "" {
		report = &kyvernov1alpha2.ClusterReportChangeRequest{}
	} else {
		report = &kyvernov1alpha2.ReportChangeRequest{}
	}
	report.SetName(name)
	report.SetNamespace(name)
	SetManagedByKyvernoLabel(report)
	return report
}
