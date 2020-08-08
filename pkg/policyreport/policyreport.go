package policyreport

import (
	v1alpha1 "github.com/kubernetes-sigs/wg-policy-prototypes/policy-report/api/v1alpha1"
	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	corev1 "k8s.io/api/core/v1"
)

// PolicyReportToPolicyViolations
func PolicyReportToPolicyViolations(reports *v1alpha1.PolicyReport, name string) *v1.PolicyViolation {
	pv := &v1.PolicyViolation{}
	status := true
	for _, report := range reports.Results {
		if report.Policy == name {
			if status {
				pv.Name = name
				pv.Spec.Name = report.Policy
				pv.Spec.Policy = report.Policy
				pv.Spec.ResourceSpec = v1.ResourceSpec{
					Name:       report.Resource.Name,
					Kind:       report.Resource.Kind,
					APIVersion: report.Resource.APIVersion,
					Namespace:  report.Resource.Namespace,
				}
				status = false
			}
			pv.Spec.ViolatedRules = append(pv.Spec.ViolatedRules, v1.ViolatedRule{
				Name:    report.Rule,
				Message: report.Message,
			})
		}
	}
	return pv
}

// ClusterPolicyReportListToClusterPolicyViolationsList
func PolicyReportListToPolicyViolationsList(reports *v1alpha1.PolicyReportList) *v1.PolicyViolationList {
	pvl := &v1.PolicyViolationList{}
	var exclude map[string]bool
	for _, report := range reports.Items {
		for _, r := range report.Results {
			if ok := exclude[r.Policy]; !ok {
				exclude[r.Policy] = true
				cpv := PolicyReportToPolicyViolations(&report,r.Policy)
				pvl.Items = append(pvl.Items,*cpv)
			}
		}
	}
	return pvl
}

// PolicyViolationsToPolicyReport
func PolicyViolationsToPolicyReport(violation *v1.PolicyViolation, reports *v1alpha1.PolicyReport) *v1alpha1.PolicyReport {
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
			var report *v1alpha1.PolicyReportResult
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
func ClusterPolicyReportToClusterPolicyViolations(reports *v1alpha1.ClusterPolicyReport, name string) *v1.ClusterPolicyViolation {
	pv := &v1.ClusterPolicyViolation{}

	for _, report := range reports.Results {
		if report.Policy == name {
				pv.Name = name
				pv.Spec.Name = report.Policy
				pv.Spec.Policy = report.Policy
				pv.Spec.ResourceSpec = v1.ResourceSpec{
					Name:       report.Resource.Name,
					Kind:       report.Resource.Kind,
					APIVersion: report.Resource.APIVersion,
					Namespace:  report.Resource.Namespace,
				}
				pv.Spec.ViolatedRules = append(pv.Spec.ViolatedRules, v1.ViolatedRule{
					Name:    report.Rule,
					Message: report.Message,
				})
		}
	}
	return pv
}

// ClusterPolicyReportListToClusterPolicyViolationsList
func ClusterPolicyReportListToClusterPolicyViolationsList(reports *v1alpha1.ClusterPolicyReportList) *v1.ClusterPolicyViolationList {
	pvl := &v1.ClusterPolicyViolationList{}
	var exclude map[string]bool
	for _, report := range reports.Items {
		for _, r := range report.Results {
			if ok := exclude[r.Policy]; !ok {
				exclude[r.Policy] = true
				cpv := ClusterPolicyReportToClusterPolicyViolations(&report,r.Policy)
				pvl.Items = append(pvl.Items,*cpv)
			}
		}
	}
	return pvl
}

// ClusterPolicyViolationsToClusterPolicyReport
func ClusterPolicyViolationsToClusterPolicyReport(violation *v1.ClusterPolicyViolation, reports *v1alpha1.ClusterPolicyReport) *v1alpha1.ClusterPolicyReport {
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
				reports.Results = append(reports.Results, report)
			}
		}
		if !status {
			var report *v1alpha1.PolicyReportResult
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
