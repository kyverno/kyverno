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

func UpdateReport(ctx context.Context, report reportsv1.ReportInterface, client versioned.Interface, orClient openreportsclient.OpenreportsV1alpha1Interface) (reportsv1.ReportInterface, error) {
	switch v := report.(type) {
	case *openreports.ReportAdapter:
		report, err := orClient.Reports(report.GetNamespace()).Update(ctx, v.Report, metav1.UpdateOptions{})
		return &openreports.ReportAdapter{Report: report}, err
	case *openreports.ClusterReportAdapter:
		report, err := orClient.ClusterReports().Update(ctx, v.ClusterReport, metav1.UpdateOptions{})
		return &openreports.ClusterReportAdapter{ClusterReport: report}, err
	case *openreports.WgpolicyClusterReportAdapter:
		wgpolr, err := client.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Update(ctx, v.ClusterPolicyReport, metav1.UpdateOptions{})
		return openreports.NewWGCpolAdapter(wgpolr), err
	case *openreports.WgpolicyReportAdapter:
		wgpolr, err := client.Wgpolicyk8sV1alpha2().PolicyReports(v.GetNamespace()).Update(ctx, v.PolicyReport, metav1.UpdateOptions{})
		return openreports.NewWGPolAdapter(wgpolr), err
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
