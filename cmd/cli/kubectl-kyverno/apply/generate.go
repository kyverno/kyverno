package apply

import (
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func mergeClusterReport(
	clustered []policyreportv1alpha2.ClusterPolicyReport,
	namespaced []policyreportv1alpha2.PolicyReport,
) policyreportv1alpha2.ClusterPolicyReport {
	var results []policyreportv1alpha2.PolicyReportResult
	for _, report := range clustered {
		results = append(results, report.Results...)
	}
	for _, report := range namespaced {
		if report.GetNamespace() != "" {
			continue
		}
		results = append(results, report.Results...)
	}
	return policyreportv1alpha2.ClusterPolicyReport{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterPolicyReport",
			APIVersion: policyreportv1alpha2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterpolicyreport,
		},
		Results: results,
		Summary: reportutils.CalculateSummary(results),
	}
}
