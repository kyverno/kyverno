package policyreport

import (
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	policyreport "github.com/nirmata/kyverno/pkg/api/policyreport/v1alpha1"
)

// RemovePolicyViolation
func RemovePolicyViolation(reports *policyreport.PolicyReport, name string) *policyreport.PolicyReport {
	pv := &policyreport.PolicyReport{}
	pv = reports
	pv.Results = []*policyreport.PolicyReportResult{}
	for _, result := range reports.Results {
		if result.Policy != name {
			pv.Results = append(pv.Results, result)
		}
	}
	return pv
}

//RemoveClusterPolicyViolation
func RemoveClusterPolicyViolation(reports *policyreport.ClusterPolicyReport, name string) *policyreport.ClusterPolicyReport {
	pv := &policyreport.ClusterPolicyReport{}
	pv = reports
	pv.Results = []*policyreport.PolicyReportResult{}
	for _, result := range reports.Results {
		if result.Policy != name {
			pv.Results = append(pv.Results, result)
		}
	}
	return pv
}

// CreatePolicyReportToPolicyReport
func CreatePolicyReportToPolicyReport(violation *kyverno.PolicyViolationTemplate, reports *policyreport.PolicyReport) *policyreport.PolicyReport {
	return &policyreport.PolicyReport{}
}

// ClusterPolicyViolationsToClusterPolicyReport
func CreateClusterPolicyViolationsToClusterPolicyReport(violation *kyverno.PolicyViolationTemplate, reports *policyreport.ClusterPolicyReport) *policyreport.ClusterPolicyReport {
	return &policyreport.ClusterPolicyReport{}
}
