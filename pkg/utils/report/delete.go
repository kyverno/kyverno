package report

import (
	"context"
	"errors"

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/openreports"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	openreportsclient "openreports.io/pkg/client/clientset/versioned/typed/openreports.io/v1alpha1"
)

func DeleteReport(ctx context.Context, report reportsv1.ReportInterface, client versioned.Interface, orClient openreportsclient.OpenreportsV1alpha1Interface) error {
	switch v := report.(type) {
	case *openreports.ReportAdapter:
		return orClient.Reports(report.GetNamespace()).Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *openreports.ClusterReportAdapter:
		return orClient.ClusterReports().Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *openreports.WgpolicyClusterReportAdapter:
		return client.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *openreports.WgpolicyReportAdapter:
		return client.Wgpolicyk8sV1alpha2().PolicyReports(v.GetNamespace()).Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *reportsv1.EphemeralReport:
		return client.ReportsV1().EphemeralReports(report.GetNamespace()).Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *reportsv1.ClusterEphemeralReport:
		return client.ReportsV1().ClusterEphemeralReports().Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	default:
		return errors.New("unknow type")
	}
}
