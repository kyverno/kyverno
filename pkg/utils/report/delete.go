package report

import (
	"context"
	"errors"

	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteReport(ctx context.Context, report kyvernov1alpha2.ReportInterface, client versioned.Interface) error {
	switch v := report.(type) {
	case *kyvernov1alpha2.AdmissionReport:
		return client.KyvernoV1alpha2().AdmissionReports(report.GetNamespace()).Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *kyvernov1alpha2.ClusterAdmissionReport:
		return client.KyvernoV1alpha2().ClusterAdmissionReports().Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *kyvernov1alpha2.BackgroundScanReport:
		return client.KyvernoV1alpha2().BackgroundScanReports(report.GetNamespace()).Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *kyvernov1alpha2.ClusterBackgroundScanReport:
		return client.KyvernoV1alpha2().ClusterBackgroundScanReports().Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *policyreportv1alpha2.PolicyReport:
		return client.Wgpolicyk8sV1alpha2().PolicyReports(report.GetNamespace()).Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *policyreportv1alpha2.ClusterPolicyReport:
		return client.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	default:
		return errors.New("unknow type")
	}
}
