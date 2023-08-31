package report

import (
	"github.com/kyverno/kyverno/api/kyverno"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	annotationsutils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/annotations"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ComputePolicyReportResultsPerPolicy(auditWarn bool, engineResponses ...engineapi.EngineResponse) map[engineapi.GenericPolicy][]policyreportv1alpha2.PolicyReportResult {
	results := map[engineapi.GenericPolicy][]policyreportv1alpha2.PolicyReportResult{}
	for _, engineResponse := range engineResponses {
		policy := engineResponse.Policy()
		policyName := policy.GetName()
		audit := engineResponse.GetValidationFailureAction().Audit()
		scored := annotationsutils.Scored(policy.GetAnnotations())
		category := annotationsutils.Category(policy.GetAnnotations())
		severity := annotationsutils.Severity(policy.GetAnnotations())
		for _, ruleResponse := range engineResponse.PolicyResponse.Rules {
			// TODO only validation is managed here ?
			if ruleResponse.RuleType() != engineapi.Validation {
				continue
			}
			result := policyreportv1alpha2.PolicyReportResult{
				// TODO policy name looks wrong, it should consider the namespace too
				Policy: policyName,
				Resources: []corev1.ObjectReference{
					{
						Kind:       engineResponse.Resource.GetKind(),
						Namespace:  engineResponse.Resource.GetNamespace(),
						APIVersion: engineResponse.Resource.GetAPIVersion(),
						Name:       engineResponse.Resource.GetName(),
						UID:        engineResponse.Resource.GetUID(),
					},
				},
				Scored:   scored,
				Category: category,
				Severity: severity,
			}
			if ruleResponse.Status() == engineapi.RuleStatusSkip {
				result.Result = policyreportv1alpha2.StatusSkip
			} else if ruleResponse.Status() == engineapi.RuleStatusError {
				result.Result = policyreportv1alpha2.StatusError
			} else if ruleResponse.Status() == engineapi.RuleStatusPass {
				result.Result = policyreportv1alpha2.StatusPass
			} else if ruleResponse.Status() == engineapi.RuleStatusFail {
				if !scored || (audit && auditWarn) {
					result.Result = policyreportv1alpha2.StatusWarn
				} else {
					result.Result = policyreportv1alpha2.StatusFail
				}
			} else {
				result.Result = policyreportv1alpha2.StatusError
			}
			if policy.GetType() == engineapi.KyvernoPolicyType {
				result.Rule = ruleResponse.Name()
			}
			result.Message = ruleResponse.Message()
			result.Source = kyverno.ValueKyvernoApp
			result.Timestamp = metav1.Timestamp{Seconds: ruleResponse.Stats().Timestamp()}
			results[policy] = append(results[policy], result)
		}
	}
	return results
}

func ComputePolicyReports(auditWarn bool, engineResponses ...engineapi.EngineResponse) ([]policyreportv1alpha2.ClusterPolicyReport, []policyreportv1alpha2.PolicyReport) {
	var clustered []policyreportv1alpha2.ClusterPolicyReport
	var namespaced []policyreportv1alpha2.PolicyReport
	perPolicyResults := (ComputePolicyReportResultsPerPolicy(auditWarn, engineResponses...))
	for policy, results := range perPolicyResults {
		if policy.GetNamespace() == "" {
			report := policyreportv1alpha2.ClusterPolicyReport{
				TypeMeta: metav1.TypeMeta{
					APIVersion: policyreportv1alpha2.SchemeGroupVersion.String(),
					Kind:       "ClusterPolicyReport",
				},
				Results: results,
				Summary: reportutils.CalculateSummary(results),
			}
			report.SetName(policy.GetName())
			clustered = append(clustered, report)
		} else {
			report := policyreportv1alpha2.PolicyReport{
				TypeMeta: metav1.TypeMeta{
					APIVersion: policyreportv1alpha2.SchemeGroupVersion.String(),
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

func MergeClusterReports(clustered []policyreportv1alpha2.ClusterPolicyReport, namespaced []policyreportv1alpha2.PolicyReport) policyreportv1alpha2.ClusterPolicyReport {
	var results []policyreportv1alpha2.PolicyReportResult
	for _, report := range clustered {
		results = append(results, report.Results...)
	}
	// TODO why this ?
	for _, report := range namespaced {
		if report.GetNamespace() != "" {
			continue
		}
		results = append(results, report.Results...)
	}
	return policyreportv1alpha2.ClusterPolicyReport{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterPolicyReport",
			APIVersion: policyreportv1alpha2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "merged",
		},
		Results: results,
		Summary: reportutils.CalculateSummary(results),
	}
}
