package report

import (
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
)

func deepCopyV1Alpha2(report kyvernov1alpha2.ReportInterface) kyvernov1alpha2.ReportInterface {
	switch v := report.(type) {
	case *kyvernov1alpha2.AdmissionReport:
		return v.DeepCopy()
	case *kyvernov1alpha2.ClusterAdmissionReport:
		return v.DeepCopy()
	case *kyvernov1alpha2.BackgroundScanReport:
		return v.DeepCopy()
	case *kyvernov1alpha2.ClusterBackgroundScanReport:
		return v.DeepCopy()
	case *policyreportv1alpha2.PolicyReport:
		return v.DeepCopy()
	case *policyreportv1alpha2.ClusterPolicyReport:
		return v.DeepCopy()
	default:
		return nil
	}
}

func deepCopyReportV1(report kyvernov1alpha2.ReportInterface) kyvernov1alpha2.ReportInterface {
	switch v := report.(type) {
	case *reportsv1.EphemeralReport:
		return v.DeepCopy()
	case *reportsv1.ClusterEphemeralReport:
		return v.DeepCopy()
	case *policyreportv1alpha2.PolicyReport:
		return v.DeepCopy()
	case *policyreportv1alpha2.ClusterPolicyReport:
		return v.DeepCopy()
	default:
		return nil
	}
}
