package apply

import (
	"fmt"
	"strings"
	"time"

	"github.com/kyverno/kyverno/api/kyverno"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	annotationsutils "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/annotations"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const clusterpolicyreport = "clusterpolicyreport"

// resps is the engine responses generated for a single policy
func buildPolicyReports(auditWarn bool, engineResponses ...engineapi.EngineResponse) ([]policyreportv1alpha2.ClusterPolicyReport, []policyreportv1alpha2.PolicyReport) {
	var clustered []policyreportv1alpha2.ClusterPolicyReport
	var namespaced []policyreportv1alpha2.PolicyReport
	resultsMap := buildPolicyResults(auditWarn, engineResponses...)
	for scope, result := range resultsMap {
		if scope == clusterpolicyreport {
			report := policyreportv1alpha2.ClusterPolicyReport{
				TypeMeta: metav1.TypeMeta{
					APIVersion: policyreportv1alpha2.SchemeGroupVersion.String(),
					Kind:       "ClusterPolicyReport",
				},
				Results: result,
				Summary: reportutils.CalculateSummary(result),
			}
			report.SetName(scope)
			clustered = append(clustered, report)
		} else {
			report := policyreportv1alpha2.PolicyReport{
				TypeMeta: metav1.TypeMeta{
					APIVersion: policyreportv1alpha2.SchemeGroupVersion.String(),
					Kind:       "PolicyReport",
				},
				Results: result,
				Summary: reportutils.CalculateSummary(result),
			}
			policyNamespace := strings.ReplaceAll(scope, "policyreport-ns-", "")
			report.SetName(scope)
			report.SetNamespace(policyNamespace)
			namespaced = append(namespaced, report)
		}
	}

	return clustered, namespaced
}

// buildPolicyResults returns a string-PolicyReportResult map
// the key of the map is one of "clusterpolicyreport", "policyreport-ns-<namespace>"
func buildPolicyResults(auditWarn bool, engineResponses ...engineapi.EngineResponse) map[string][]policyreportv1alpha2.PolicyReportResult {
	results := make(map[string][]policyreportv1alpha2.PolicyReportResult)
	now := metav1.Timestamp{Seconds: time.Now().Unix()}

	for _, engineResponse := range engineResponses {
		policy := engineResponse.Policy()
		policyName := policy.GetName()
		policyNamespace := policy.GetNamespace()
		scored := annotationsutils.Scored(policy.GetAnnotations())
		category := annotationsutils.Category(policy.GetAnnotations())
		severity := annotationsutils.Severity(policy.GetAnnotations())

		var appname string
		if policyNamespace != "" {
			appname = fmt.Sprintf("policyreport-ns-%s", policyNamespace)
		} else {
			appname = clusterpolicyreport
		}

		for _, ruleResponse := range engineResponse.PolicyResponse.Rules {
			if ruleResponse.RuleType() != engineapi.Validation {
				continue
			}

			result := policyreportv1alpha2.PolicyReportResult{
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
				Scored:   true,
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
				if !scored {
					result.Result = policyreportv1alpha2.StatusWarn
				} else if auditWarn && engineResponse.GetValidationFailureAction().Audit() {
					result.Result = policyreportv1alpha2.StatusWarn
				} else {
					result.Result = policyreportv1alpha2.StatusFail
				}
			} else {
				fmt.Println(ruleResponse)
			}
			if policy.GetType() == engineapi.KyvernoPolicyType {
				result.Rule = ruleResponse.Name()
			}
			result.Message = ruleResponse.Message()
			result.Source = kyverno.ValueKyvernoApp
			result.Timestamp = now
			results[appname] = append(results[appname], result)
		}
	}

	return results
}
