package report

import (
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
)

func DeepCopy(report kyvernov1alpha2.ReportChangeRequestInterface) kyvernov1alpha2.ReportChangeRequestInterface {
	switch v := report.(type) {
	case *kyvernov1alpha2.AdmissionReport:
		return v.DeepCopy()
	case *kyvernov1alpha2.ClusterAdmissionReport:
		return v.DeepCopy()
	case *kyvernov1alpha2.BackgroundScanReport:
		return v.DeepCopy()
	case *kyvernov1alpha2.ClusterBackgroundScanReport:
		return v.DeepCopy()
	case *kyvernov1alpha2.ReportChangeRequest:
		return v.DeepCopy()
	case *kyvernov1alpha2.ClusterReportChangeRequest:
		return v.DeepCopy()
	default:
		return nil
	}
}
