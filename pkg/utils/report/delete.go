package report

import (
	"context"
	"errors"

	kyvernoreports "github.com/kyverno/kyverno/api/kyverno/reports/v1"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteReport(ctx context.Context, report kyvernoreports.ReportInterface, client versioned.Interface) error {
	switch v := report.(type) {
	case *kyvernoreports.AdmissionReport:
		return client.ReportsV1().AdmissionReports(report.GetNamespace()).Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *kyvernoreports.ClusterAdmissionReport:
		return client.ReportsV1().ClusterAdmissionReports().Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *kyvernoreports.BackgroundScanReport:
		return client.ReportsV1().BackgroundScanReports(report.GetNamespace()).Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *kyvernoreports.ClusterBackgroundScanReport:
		return client.ReportsV1().ClusterBackgroundScanReports().Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *policyreportv1alpha2.PolicyReport:
		return client.Wgpolicyk8sV1alpha2().PolicyReports(report.GetNamespace()).Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *policyreportv1alpha2.ClusterPolicyReport:
		return client.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	default:
		return errors.New("unknow type")
	}
}
