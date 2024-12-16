package report

import (
	policyreportv1beta1 "github.com/kyverno/kyverno/api/policyreport/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func ComputePolicyReportResult(auditWarn bool, engineResponse engineapi.EngineResponse, ruleResponse engineapi.RuleResponse) policyreportv1beta1.PolicyReportResult {
	policy := engineResponse.Policy()
	policyType := policy.GetType()
	policyName := cache.MetaObjectToName(policy.MetaObject()).String()
	resource := engineResponse.Resource
	resorceRef := &corev1.ObjectReference{
		Kind:            resource.GetKind(),
		Name:            resource.GetName(),
		Namespace:       resource.GetNamespace(),
		UID:             resource.GetUID(),
		APIVersion:      resource.GetAPIVersion(),
		ResourceVersion: resource.GetResourceVersion(),
	}

	result := reportutils.ToPolicyReportResult(policyType, policyName, ruleResponse, policy.GetAnnotations(), resorceRef)
	if result.Result == policyreportv1beta1.StatusFail {
		audit := engineResponse.GetValidationFailureAction().Audit()
		if audit && auditWarn {
			result.Result = policyreportv1beta1.StatusWarn
		}
	}

	return result
}

func ComputePolicyReportResultsPerPolicy(auditWarn bool, engineResponses ...engineapi.EngineResponse) map[engineapi.GenericPolicy][]policyreportv1beta1.PolicyReportResult {
	results := map[engineapi.GenericPolicy][]policyreportv1beta1.PolicyReportResult{}
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

func ComputePolicyReports(auditWarn bool, engineResponses ...engineapi.EngineResponse) ([]policyreportv1beta1.ClusterPolicyReport, []policyreportv1beta1.PolicyReport) {
	var clustered []policyreportv1beta1.ClusterPolicyReport
	var namespaced []policyreportv1beta1.PolicyReport
	perPolicyResults := ComputePolicyReportResultsPerPolicy(auditWarn, engineResponses...)
	for policy, results := range perPolicyResults {
		if policy.GetNamespace() == "" {
			report := policyreportv1beta1.ClusterPolicyReport{
				TypeMeta: metav1.TypeMeta{
					APIVersion: policyreportv1beta1.SchemeGroupVersion.String(),
					Kind:       "ClusterPolicyReport",
				},
				Results: results,
				Summary: reportutils.CalculateSummary(results),
			}
			report.SetName(policy.GetName())
			clustered = append(clustered, report)
		} else {
			report := policyreportv1beta1.PolicyReport{
				TypeMeta: metav1.TypeMeta{
					APIVersion: policyreportv1beta1.SchemeGroupVersion.String(),
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

func MergeClusterReports(clustered []policyreportv1beta1.ClusterPolicyReport) policyreportv1beta1.ClusterPolicyReport {
	var results []policyreportv1beta1.PolicyReportResult
	for _, report := range clustered {
		results = append(results, report.Results...)
	}
	return policyreportv1beta1.ClusterPolicyReport{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterPolicyReport",
			APIVersion: policyreportv1beta1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "merged",
		},
		Results: results,
		Summary: reportutils.CalculateSummary(results),
	}
}
