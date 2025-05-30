package report

import (
	"context"
	"errors"

	reportv1alpha1 "github.com/kyverno/kyverno/api/openreports.io/v1alpha1"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UpdateReport(ctx context.Context, report reportsv1.ReportInterface, client versioned.Interface) (reportsv1.ReportInterface, error) {
	switch v := report.(type) {
	case *reportv1alpha1.Report:
		report, err := client.OpenreportsV1alpha1().Reports(report.GetNamespace()).Update(ctx, v, metav1.UpdateOptions{})
		return report, err
	case *reportv1alpha1.ClusterReport:
		report, err := client.OpenreportsV1alpha1().ClusterReports().Update(ctx, v, metav1.UpdateOptions{})
		return report, err
	case *reportsv1.EphemeralReport:
		report, err := client.ReportsV1().EphemeralReports(report.GetNamespace()).Update(ctx, v, metav1.UpdateOptions{})
		return report, err
	case *reportsv1.ClusterEphemeralReport:
		report, err := client.ReportsV1().ClusterEphemeralReports().Update(ctx, v, metav1.UpdateOptions{})
		return report, err
	default:
		return nil, errors.New("unknow type")
	}
}
