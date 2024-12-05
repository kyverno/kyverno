package report

import (
	"context"
	"errors"

	policyreportv1beta1 "github.com/kyverno/kyverno/api/policyreport/v1beta1"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteReport(ctx context.Context, report reportsv1.ReportInterface, client versioned.Interface) error {
	switch v := report.(type) {
	case *policyreportv1beta1.PolicyReport:
		return client.Wgpolicyk8sV1beta1().PolicyReports(report.GetNamespace()).Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *policyreportv1beta1.ClusterPolicyReport:
		return client.Wgpolicyk8sV1beta1().ClusterPolicyReports().Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *reportsv1.EphemeralReport:
		return client.ReportsV1().EphemeralReports(report.GetNamespace()).Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *reportsv1.ClusterEphemeralReport:
		return client.ReportsV1().ClusterEphemeralReports().Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	default:
		return errors.New("unknow type")
	}
}
