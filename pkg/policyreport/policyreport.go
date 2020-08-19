package policyreport

import (
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	corev1 "k8s.io/api/core/v1"
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

// CreatePolicyViolationToPolicyReport
func CreatePolicyViolationToPolicyReport(violation *kyverno.PolicyViolationTemplate, reports *policyreport.PolicyReport) *policyreport.PolicyReport {
	for _,result := range reports.Results {
		for i,rule := range violation.Spec.ViolatedRules {
			if result.Policy == violation.Spec.Policy && result.Rule == rule.Name {
				result.Message = rule.Message
				violation.Spec.ViolatedRules = append(violation.Spec.ViolatedRules[:i], violation.Spec.ViolatedRules[i+1:]...)
			}
		}
	}
	for _,rule := range violation.Spec.ViolatedRules {
		result := &policyreport.PolicyReportResult{
			Policy: violation.Spec.Policy,
			Rule : rule.Name,
			Message: rule.Message,
			Resource: &corev1.ObjectReference{
				Kind : violation.Spec.Kind,
				Namespace:violation.Spec.Namespace,
				APIVersion:violation.Spec.APIVersion,
				Name:violation.Spec.Name,
			},
		}
		reports.Results =  append(reports.Results,result)
	}
	return reports
}

// ClusterPolicyViolationsToClusterPolicyReport
func CreateClusterPolicyViolationsToClusterPolicyReport(violation *kyverno.PolicyViolationTemplate, reports *policyreport.ClusterPolicyReport) *policyreport.ClusterPolicyReport {
	pv := &policyreport.ClusterPolicyReport{}
	for _,result := range pv.Results {
		for i,rule := range violation.Spec.ViolatedRules {
			if result.Policy == violation.Spec.Policy && result.Rule == rule.Name {
				result.Message = rule.Message
				violation.Spec.ViolatedRules = append(violation.Spec.ViolatedRules[:i], violation.Spec.ViolatedRules[i+1:]...)
			}
		}
	}
	for _,rule := range violation.Spec.ViolatedRules {
		result := &policyreport.PolicyReportResult{
			Policy: violation.Spec.Policy,
			Rule : rule.Name,
			Message: rule.Message,
			Resource: &corev1.ObjectReference{
				Kind : violation.Spec.Kind,
				Namespace:violation.Spec.Namespace,
				APIVersion:violation.Spec.APIVersion,
				Name:violation.Spec.Name,
			},
		}
		pv.Results =  append(pv.Results,result)
	}
	return pv
}
