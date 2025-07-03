package report

import (
	"context"
	"errors"

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateEphemeralReport(ctx context.Context, report reportsv1.ReportInterface, client versioned.Interface) (reportsv1.ReportInterface, error) {
	switch v := report.(type) {
	case *reportsv1.EphemeralReport:
		report, err := client.ReportsV1().EphemeralReports(report.GetNamespace()).Create(ctx, v, metav1.CreateOptions{})
		return report, err
	case *reportsv1.ClusterEphemeralReport:
		report, err := client.ReportsV1().ClusterEphemeralReports().Create(ctx, v, metav1.CreateOptions{})
		return report, err
	default:
		return nil, errors.New("unknow type")
	}
}
