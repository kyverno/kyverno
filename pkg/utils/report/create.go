package report

import (
	"context"
	"errors"

	kyvernoreports "github.com/kyverno/kyverno/api/kyverno/reports/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateReport(ctx context.Context, report kyvernov1alpha2.ReportInterface, client versioned.Interface) (kyvernov1alpha2.ReportInterface, error) {
	switch v := report.(type) {
	case *kyvernoreports.AdmissionReport:
		report, err := client.ReportsV1().AdmissionReports(report.GetNamespace()).Create(ctx, v, metav1.CreateOptions{})
		return report, err
	case *kyvernoreports.ClusterAdmissionReport:
		report, err := client.ReportsV1().ClusterAdmissionReports().Create(ctx, v, metav1.CreateOptions{})
		return report, err
	case *kyvernoreports.BackgroundScanReport:
		report, err := client.ReportsV1().BackgroundScanReports(report.GetNamespace()).Create(ctx, v, metav1.CreateOptions{})
		return report, err
	case *kyvernoreports.ClusterBackgroundScanReport:
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
