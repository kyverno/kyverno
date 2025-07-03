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

func CreatePermanentReport(ctx context.Context, report reportsv1.ReportInterface, client versioned.Interface, orClient openreportsclient.OpenreportsV1alpha1Interface) (reportsv1.ReportInterface, error) {
	switch v := report.(type) {
	case *openreports.ReportAdapter:
		report, err := orClient.Reports(report.GetNamespace()).Create(ctx, v.Report, metav1.CreateOptions{})
		return &openreports.ReportAdapter{Report: report}, err
	case *openreports.ClusterReportAdapter:
		report, err := orClient.ClusterReports().Create(ctx, v.ClusterReport, metav1.CreateOptions{})
		return &openreports.ClusterReportAdapter{ClusterReport: report}, err
	case *openreports.WgpolicyReportAdapter:
		report, err := client.Wgpolicyk8sV1alpha2().PolicyReports(report.GetNamespace()).Create(ctx, v.PolicyReport, metav1.CreateOptions{})
		return openreports.NewWGPolAdapter(report), err
	case *openreports.WgpolicyClusterReportAdapter:
		report, err := client.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Create(ctx, v.ClusterPolicyReport, metav1.CreateOptions{})
		return openreports.NewWGCpolAdapter(report), err
	default:
		return nil, errors.New("unknow type")
	}
}
