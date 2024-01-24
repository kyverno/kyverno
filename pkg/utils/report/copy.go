package report

import (
	kyvernoreports "github.com/kyverno/kyverno/api/kyverno/reports/v1"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
)

func DeepCopy(report kyvernoreports.ReportInterface) kyvernoreports.ReportInterface {
	switch v := report.(type) {
	case *kyvernoreports.AdmissionReport:
		return v.DeepCopy()
	case *kyvernoreports.ClusterAdmissionReport:
		return v.DeepCopy()
	case *kyvernoreports.BackgroundScanReport:
		return v.DeepCopy()
	case *kyvernoreports.ClusterBackgroundScanReport:
		return v.DeepCopy()
	case *policyreportv1alpha2.PolicyReport:
		return v.DeepCopy()
	case *policyreportv1alpha2.ClusterPolicyReport:
		return v.DeepCopy()
	default:
		return nil
	}
}
