package report

import (
	"context"
	"errors"

	reportv1alpha1 "github.com/kyverno/kyverno/api/openreports.io/v1alpha1"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteReport(ctx context.Context, report reportsv1.ReportInterface, client versioned.Interface) error {
	switch v := report.(type) {
	case *reportv1alpha1.Report:
		return client.OpenreportsV1alpha1().Reports(report.GetNamespace()).Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *reportv1alpha1.ClusterReport:
		return client.OpenreportsV1alpha1().ClusterReports().Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *reportsv1.EphemeralReport:
		return client.ReportsV1().EphemeralReports(report.GetNamespace()).Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	case *reportsv1.ClusterEphemeralReport:
		return client.ReportsV1().ClusterEphemeralReports().Delete(ctx, v.GetName(), metav1.DeleteOptions{})
	default:
		return errors.New("unknow type")
	}
}
