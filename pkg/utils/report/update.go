package report

import (
	"context"
	"errors"

	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UpdateReport(ctx context.Context, report kyvernov1alpha2.ReportInterface, client versioned.Interface) (kyvernov1alpha2.ReportInterface, error) {
	switch v := report.(type) {
	case *kyvernov1alpha2.AdmissionReport:
		report, err := client.KyvernoV1alpha2().AdmissionReports(report.GetNamespace()).Update(ctx, v, metav1.UpdateOptions{})
		return report, err
	case *kyvernov1alpha2.ClusterAdmissionReport:
		report, err := client.KyvernoV1alpha2().ClusterAdmissionReports().Update(ctx, v, metav1.UpdateOptions{})
		return report, err
	case *kyvernov1alpha2.BackgroundScanReport:
		report, err := client.KyvernoV1alpha2().BackgroundScanReports(report.GetNamespace()).Update(ctx, v, metav1.UpdateOptions{})
		return report, err
	case *kyvernov1alpha2.ClusterBackgroundScanReport:
		report, err := client.KyvernoV1alpha2().ClusterBackgroundScanReports().Update(ctx, v, metav1.UpdateOptions{})
		return report, err
	case *policyreportv1alpha2.PolicyReport:
		report, err := client.Wgpolicyk8sV1alpha2().PolicyReports(report.GetNamespace()).Update(ctx, v, metav1.UpdateOptions{})
		return report, err
	case *policyreportv1alpha2.ClusterPolicyReport:
		report, err := client.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Update(ctx, v, metav1.UpdateOptions{})
		return report, err
	default:
		return nil, errors.New("unknow type")
	}
}
