package policyreport

import (
	policyreport "github.com/nirmata/kyverno/pkg/api/policyreport/v1alpha1"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	corev1 "k8s.io/api/core/v1"
)

// PolicyReportToPolicyViolations
func PolicyReportToPolicyViolations(reports *policyreport.PolicyReport, name string) *kyverno.PolicyViolation {
	pv := &kyverno.PolicyViolation{}
	status := true
	for _, report := range reports.Results {
		if report.Policy == name {
			if status {
				pv.Name = name
				pv.Spec.Name = report.Policy
				pv.Spec.Policy = report.Policy
				pv.Spec.ResourceSpec = kyverno.ResourceSpec{
					Name:       report.Resource.Name,
					Kind:       report.Resource.Kind,
					APIVersion: report.Resource.APIVersion,
					Namespace:  report.Resource.Namespace,
				}
				status = false
			}
			pv.Spec.ViolatedRules = append(pv.Spec.ViolatedRules, kyverno.ViolatedRule{
				Name:    report.Rule,
				Message: report.Message,
			})
		}
	}
	return pv
}


// PolicyViolationsToPolicyReport
func PolicyViolationsToPolicyReport(violation *kyverno.PolicyViolationTemplate, reports *policyreport.PolicyReport) *policyreport.PolicyReport {
	for _, rule := range violation.Spec.ViolatedRules {
		status := true
		for _, report := range reports.Results {
			if report.Policy == violation.Spec.Policy {
				report.Policy = violation.Spec.Policy
				report.Resource = &corev1.ObjectReference{
					Kind:       violation.Spec.ResourceSpec.Kind,
					Name:       violation.Spec.ResourceSpec.Name,
					APIVersion: violation.Spec.ResourceSpec.APIVersion,
					Namespace:  violation.Spec.ResourceSpec.Namespace,
				}
				report.Message = rule.Message
				report.Rule = rule.Name
				status = false
			}
		}
		if !status {
			var report *policyreport.PolicyReportResult
			report.Policy = violation.Spec.Policy
			report.Resource = &corev1.ObjectReference{
				Kind:       violation.Spec.ResourceSpec.Kind,
				Name:       violation.Spec.ResourceSpec.Name,
				APIVersion: violation.Spec.ResourceSpec.APIVersion,
				Namespace:  violation.Spec.ResourceSpec.Namespace,
			}
			report.Message = rule.Message
			report.Rule = rule.Name
			reports.Results = append(reports.Results, report)
		}
	}
	return reports
}

// ClusterPolicyReportToClusterPolicyViolations
func ClusterPolicyReportToClusterPolicyViolations(reports *policyreport.ClusterPolicyReport, name string) *kyverno.ClusterPolicyViolation {
	pv := &kyverno.ClusterPolicyViolation{}

	for _, report := range reports.Results {
		if report.Policy == name {
				pv.Name = name
				pv.Spec.Name = report.Policy
				pv.Spec.Policy = report.Policy
				pv.Spec.ResourceSpec = kyverno.ResourceSpec{
					Name:       report.Resource.Name,
					Kind:       report.Resource.Kind,
					APIVersion: report.Resource.APIVersion,
					Namespace:  report.Resource.Namespace,
				}
				pv.Spec.ViolatedRules = append(pv.Spec.ViolatedRules, kyverno.ViolatedRule{
					Name:    report.Rule,
					Message: report.Message,
				})
		}
	}
	return pv
}

// ClusterPolicyViolationsToClusterPolicyReport
func ClusterPolicyViolationsToClusterPolicyReport(violation *kyverno.PolicyViolationTemplate, reports *policyreport.ClusterPolicyReport) *policyreport.ClusterPolicyReport {
	for _, rule := range violation.Spec.ViolatedRules {
		status := false
		for _, report := range reports.Results {
			if report.Policy == violation.Spec.Policy {
				report.Policy = violation.Spec.Policy
				report.Resource = &corev1.ObjectReference{
					Kind:       violation.Spec.ResourceSpec.Kind,
					Name:       violation.Spec.ResourceSpec.Name,
					APIVersion: violation.Spec.ResourceSpec.APIVersion,
				}
				report.Message = rule.Message
				report.Rule = rule.Name
				status = true
			}
		}
		if !status {
			var report *policyreport.PolicyReportResult
			report.Policy = violation.Spec.Policy
			report.Resource = &corev1.ObjectReference{
				Kind:       violation.Spec.ResourceSpec.Kind,
				Name:       violation.Spec.ResourceSpec.Name,
				APIVersion: violation.Spec.ResourceSpec.APIVersion,
				Namespace:  violation.Spec.ResourceSpec.Namespace,
			}
			report.Message = rule.Message
			report.Rule = rule.Name
			reports.Results = append(reports.Results, report)
		}
	}
	return reports
}
