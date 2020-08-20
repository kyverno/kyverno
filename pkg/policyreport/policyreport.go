package policyreport

import (
	"fmt"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	policyreport "github.com/nirmata/kyverno/pkg/api/policyreport/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// RemovePolicyViolation
func RemovePolicyViolation(reports *policyreport.PolicyReport, name string) *policyreport.PolicyReport {
	for i, result := range reports.Results {
		if result.Resource.Name == name {
			reports.Results = append(reports.Results[:i], reports.Results[i+1:]...)
		}
	}
	return reports
}

//RemoveClusterPolicyViolation
func RemoveClusterPolicyViolation(reports *policyreport.ClusterPolicyReport, name string) *policyreport.ClusterPolicyReport {
	for i, result := range reports.Results {
		if result.Resource.Name == name {
			reports.Results = append(reports.Results[:i], reports.Results[i+1:]...)
		}
	}
	return reports
}

// CreatePolicyViolationToPolicyReport
func CreatePolicyViolationToPolicyReport(violation *kyverno.PolicyViolationTemplate, reports *policyreport.PolicyReport) *policyreport.PolicyReport {
	for _, result := range reports.Results {
		for i, rule := range violation.Spec.ViolatedRules {
			if result.Policy == violation.Spec.Policy && result.Rule == rule.Name && result.Resource.Name == violation.Spec.Name {
				result.Message = rule.Message
				violation.Spec.ViolatedRules = append(violation.Spec.ViolatedRules[:i], violation.Spec.ViolatedRules[i+1:]...)
			}
		}
	}
	for _, rule := range violation.Spec.ViolatedRules {
		result := &policyreport.PolicyReportResult{
			Policy:  violation.Spec.Policy,
			Rule:    rule.Name,
			Message: rule.Message,
			Resource: &corev1.ObjectReference{
				Kind:       violation.Spec.Kind,
				Namespace:  violation.Spec.Namespace,
				APIVersion: violation.Spec.APIVersion,
				Name:       violation.Spec.Name,
			},
		}
		reports.Results = append(reports.Results, result)
		switch rule.Check {
		case "pass":
			reports.Summary.Pass++
			break
		case "fail":
			reports.Summary.Fail++
			break
		default:
			reports.Summary.Skip++
			break
		}
	}
	return reports
}

// ClusterPolicyViolationsToClusterPolicyReport
func CreateClusterPolicyViolationsToClusterPolicyReport(violation *kyverno.PolicyViolationTemplate, reports *policyreport.ClusterPolicyReport) *policyreport.ClusterPolicyReport {
	pv := &policyreport.ClusterPolicyReport{}
	for _, result := range pv.Results {
		for i, rule := range violation.Spec.ViolatedRules {
			if result.Policy == violation.Spec.Policy && result.Rule == rule.Name {
				result.Message = rule.Message
				violation.Spec.ViolatedRules = append(violation.Spec.ViolatedRules[:i], violation.Spec.ViolatedRules[i+1:]...)
			}
		}
	}
	for _, rule := range violation.Spec.ViolatedRules {
		result := &policyreport.PolicyReportResult{
			Policy:  violation.Spec.Policy,
			Rule:    rule.Name,
			Message: rule.Message,
			Resource: &corev1.ObjectReference{
				Kind:       violation.Spec.Kind,
				Namespace:  violation.Spec.Namespace,
				APIVersion: violation.Spec.APIVersion,
				Name:       violation.Spec.Name,
			},
		}
		pv.Results = append(pv.Results, result)
		fmt.Println(rule.Check)
		switch rule.Check {
		case "pass":
			reports.Summary.Pass++
			break
		case "fail":
			reports.Summary.Fail++
			break
		default:
			reports.Summary.Skip++
			break
		}
	}
	return pv
}
