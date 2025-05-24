package report

import (
	reportv1alpha1 "github.com/kyverno/kyverno/api/openreports.io/v1alpha1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ComputePolicyReportResult(auditWarn bool, engineResponse engineapi.EngineResponse, ruleResponse engineapi.RuleResponse) reportv1alpha1.ReportResult {
	resource := engineResponse.Resource
	resorceRef := &corev1.ObjectReference{
		Kind:            resource.GetKind(),
		Name:            resource.GetName(),
		Namespace:       resource.GetNamespace(),
		UID:             resource.GetUID(),
		APIVersion:      resource.GetAPIVersion(),
		ResourceVersion: resource.GetResourceVersion(),
	}
	result := reportutils.ToPolicyReportResult(engineResponse.Policy(), ruleResponse, resorceRef)
	if result.Result == reportv1alpha1.StatusFail {
		audit := engineResponse.GetValidationFailureAction().Audit()
		if audit && auditWarn {
			result.Result = reportv1alpha1.StatusWarn
		}
	}

	return result
}

func ComputePolicyReportResultsPerPolicy(auditWarn bool, engineResponses ...engineapi.EngineResponse) map[engineapi.GenericPolicy][]reportv1alpha1.ReportResult {
	results := map[engineapi.GenericPolicy][]reportv1alpha1.ReportResult{}
	for _, engineResponse := range engineResponses {
		if len(engineResponse.PolicyResponse.Rules) == 0 {
			continue
		}
		policy := engineResponse.Policy()
		for _, ruleResponse := range engineResponse.PolicyResponse.Rules {
			// TODO only validation is managed here ?
			// if ruleResponse.RuleType() != engineapi.Validation && ruleResponse.RuleType() != engineapi.ImageVerify {
			// 	continue
			// }
			results[policy] = append(results[policy], ComputePolicyReportResult(auditWarn, engineResponse, ruleResponse))
		}
	}
	if len(results) == 0 {
		return nil
	}
	return results
}

func ComputePolicyReports(auditWarn bool, engineResponses ...engineapi.EngineResponse) ([]reportv1alpha1.ClusterReport, []reportv1alpha1.Report) {
	var clustered []reportv1alpha1.ClusterReport
	var namespaced []reportv1alpha1.Report
	perPolicyResults := ComputePolicyReportResultsPerPolicy(auditWarn, engineResponses...)
	for policy, results := range perPolicyResults {
		if policy.GetNamespace() == "" {
			report := reportv1alpha1.ClusterReport{
				TypeMeta: metav1.TypeMeta{
					APIVersion: reportv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ClusterPolicyReport",
				},
				Results: results,
				Summary: reportutils.CalculateSummary(results),
			}
			report.SetName(policy.GetName())
			clustered = append(clustered, report)
		} else {
			report := reportv1alpha1.Report{
				TypeMeta: metav1.TypeMeta{
					APIVersion: reportv1alpha1.SchemeGroupVersion.String(),
					Kind:       "PolicyReport",
				},
				Results: results,
				Summary: reportutils.CalculateSummary(results),
			}
			report.SetName(policy.GetName())
			report.SetNamespace(policy.GetNamespace())
			namespaced = append(namespaced, report)
		}
	}
	return clustered, namespaced
}

func MergeClusterReports(clustered []reportv1alpha1.ClusterReport) reportv1alpha1.ClusterReport {
	var results []reportv1alpha1.ReportResult
	for _, report := range clustered {
		results = append(results, report.Results...)
	}
	return reportv1alpha1.ClusterReport{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterPolicyReport",
			APIVersion: reportv1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "merged",
		},
		Results: results,
		Summary: reportutils.CalculateSummary(results),
	}
}
