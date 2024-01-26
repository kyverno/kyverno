package report

import (
	"context"
	"errors"

	reportv1 "github.com/kyverno/kyverno/api/kyverno/reports/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createV1Alpha1Report(ctx context.Context, report kyvernov1alpha2.ReportInterface, client versioned.Interface) (kyvernov1alpha2.ReportInterface, error) {
	switch v := report.(type) {
	case *kyvernov1alpha2.AdmissionReport:
		report, err := client.KyvernoV1alpha2().AdmissionReports(report.GetNamespace()).Create(ctx, v, metav1.CreateOptions{})
		return report, err
	case *kyvernov1alpha2.ClusterAdmissionReport:
		report, err := client.KyvernoV1alpha2().ClusterAdmissionReports().Create(ctx, v, metav1.CreateOptions{})
		return report, err
	case *kyvernov1alpha2.BackgroundScanReport:
		report, err := client.KyvernoV1alpha2().BackgroundScanReports(report.GetNamespace()).Create(ctx, v, metav1.CreateOptions{})
		return report, err
	case *kyvernov1alpha2.ClusterBackgroundScanReport:
		report, err := client.KyvernoV1alpha2().ClusterBackgroundScanReports().Create(ctx, v, metav1.CreateOptions{})
		return report, err
	case *policyreportv1alpha2.PolicyReport:
		report, err := client.Wgpolicyk8sV1alpha2().PolicyReports(report.GetNamespace()).Create(ctx, v, metav1.CreateOptions{})
		return report, err
	case *policyreportv1alpha2.ClusterPolicyReport:
		report, err := client.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Create(ctx, v, metav1.CreateOptions{})
		return report, err
	default:
		return nil, errors.New("unknow type")
	}
}

func createReportV1Report(ctx context.Context, report kyvernov1alpha2.ReportInterface, client versioned.Interface) (kyvernov1alpha2.ReportInterface, error) {
	switch v := report.(type) {
	case *reportv1.AdmissionReport:
		report, err := client.ReportsV1().AdmissionReports(report.GetNamespace()).Create(ctx, v, metav1.CreateOptions{})
		return report, err
	case *reportv1.ClusterAdmissionReport:
		report, err := client.ReportsV1().ClusterAdmissionReports().Create(ctx, v, metav1.CreateOptions{})
		return report, err
	case *reportv1.BackgroundScanReport:
		report, err := client.ReportsV1().BackgroundScanReports(report.GetNamespace()).Create(ctx, v, metav1.CreateOptions{})
		return report, err
	case *reportv1.ClusterBackgroundScanReport:
		report, err := client.ReportsV1().ClusterBackgroundScanReports().Create(ctx, v, metav1.CreateOptions{})
		return report, err
	case *policyreportv1alpha2.PolicyReport:
		report, err := client.Wgpolicyk8sV1alpha2().PolicyReports(report.GetNamespace()).Create(ctx, v, metav1.CreateOptions{})
		return report, err
	case *policyreportv1alpha2.ClusterPolicyReport:
		report, err := client.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Create(ctx, v, metav1.CreateOptions{})
		return report, err
	default:
		return nil, errors.New("unknow type")
	}
}
