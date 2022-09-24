package report

import (
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
)

func DeepCopy(report kyvernov1alpha2.ReportInterface) kyvernov1alpha2.ReportInterface {
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
