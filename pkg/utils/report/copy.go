package report

import (
	reportv1alpha1 "github.com/kyverno/kyverno/api/openreports.io/v1alpha1"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
)

func DeepCopy(report reportsv1.ReportInterface) reportsv1.ReportInterface {
	switch v := report.(type) {
	case *reportv1alpha1.Report:
		return v.DeepCopy()
	case *reportv1alpha1.ClusterReport:
		return v.DeepCopy()
	case *reportsv1.EphemeralReport:
		return v.DeepCopy()
	case *reportsv1.ClusterEphemeralReport:
		return v.DeepCopy()
	default:
		return nil
	}
}
