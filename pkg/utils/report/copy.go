package report

import (
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/openreports"
)

func DeepCopy(report reportsv1.ReportInterface) reportsv1.ReportInterface {
	switch v := report.(type) {
	case *openreports.ReportAdapter:
		return &openreports.ReportAdapter{Report: v.DeepCopy()}
	case *openreports.ClusterReportAdapter:
		return &openreports.ClusterReportAdapter{ClusterReport: v.DeepCopy()}
	case *openreports.WgpolicyReportAdapter:
		return openreports.NewWGPolAdapter(v.DeepCopy())
	case *openreports.WgpolicyClusterReportAdapter:
		return openreports.NewWGCpolAdapter(v.DeepCopy())
	case *reportsv1.EphemeralReport:
		return v.DeepCopy()
	case *reportsv1.ClusterEphemeralReport:
		return v.DeepCopy()
	default:
		return nil
	}
}
