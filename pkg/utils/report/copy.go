package report

import (
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
)

func DeepCopy(report reportsv1.ReportInterface) reportsv1.ReportInterface {
	switch v := report.(type) {
	case *policyreportv1alpha2.PolicyReport:
		return v.DeepCopy()
	case *policyreportv1alpha2.ClusterPolicyReport:
		return v.DeepCopy()
	case *reportsv1.EphemeralReport:
		return v.DeepCopy()
	case *reportsv1.ClusterEphemeralReport:
		return v.DeepCopy()
	default:
		return nil
	}
}
