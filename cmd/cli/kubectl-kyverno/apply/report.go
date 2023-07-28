package apply

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const clusterpolicyreport = "clusterpolicyreport"

// resps is the engine responses generated for a single policy
func buildPolicyReports(auditWarn bool, engineResponses ...engineapi.EngineResponse) (res []*unstructured.Unstructured) {
	var raw []byte
	var err error

	resultsMap := buildPolicyResults(auditWarn, engineResponses...)
	for scope, result := range resultsMap {
		if scope == clusterpolicyreport {
			report := &policyreportv1alpha2.ClusterPolicyReport{
				TypeMeta: metav1.TypeMeta{
					APIVersion: policyreportv1alpha2.SchemeGroupVersion.String(),
					Kind:       "ClusterPolicyReport",
				},
				Results: result,
				Summary: calculateSummary(result),
			}

			report.SetName(scope)
			if raw, err = json.Marshal(report); err != nil {
				log.Log.V(3).Info("failed to serialize policy report", "name", report.Name, "scope", scope, "error", err)
			}
		} else {
			report := &policyreportv1alpha2.PolicyReport{
				TypeMeta: metav1.TypeMeta{
					APIVersion: policyreportv1alpha2.SchemeGroupVersion.String(),
					Kind:       "PolicyReport",
				},
				Results: result,
				Summary: calculateSummary(result),
			}

			ns := strings.ReplaceAll(scope, "policyreport-ns-", "")
			report.SetName(scope)
			report.SetNamespace(ns)

			if raw, err = json.Marshal(report); err != nil {
				log.Log.V(3).Info("failed to serialize policy report", "name", report.Name, "scope", scope, "error", err)
			}
		}

		reportUnstructured, err := kubeutils.BytesToUnstructured(raw)
		if err != nil {
			log.Log.V(3).Info("failed to convert policy report", "scope", scope, "error", err)
			continue
		}

		res = append(res, reportUnstructured)
	}

	return
}

// buildPolicyResults returns a string-PolicyReportResult map
// the key of the map is one of "clusterpolicyreport", "policyreport-ns-<namespace>"
func buildPolicyResults(auditWarn bool, engineResponses ...engineapi.EngineResponse) map[string][]policyreportv1alpha2.PolicyReportResult {
	results := make(map[string][]policyreportv1alpha2.PolicyReportResult)
	now := metav1.Timestamp{Seconds: time.Now().Unix()}

	for _, engineResponse := range engineResponses {
		policy := engineResponse.Policy()
		var appname string
		ns := policy.GetNamespace()
		ann := policy.GetAnnotations()
		if ns != "" {
			appname = fmt.Sprintf("policyreport-ns-%s", ns)
		} else {
			appname = clusterpolicyreport
		}

		for _, ruleResponse := range engineResponse.PolicyResponse.Rules {
			if ruleResponse.RuleType() != engineapi.Validation {
				continue
			}

			result := policyreportv1alpha2.PolicyReportResult{
				Policy: policy.GetName(),
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
				Category: ann[kyvernov1.AnnotationPolicyCategory],
				Severity: reportutils.SeverityFromString(ann[kyvernov1.AnnotationPolicySeverity]),
			}

			ann := engineResponse.Policy().GetAnnotations()
			if ruleResponse.Status() == engineapi.RuleStatusSkip {
				result.Result = policyreportv1alpha2.StatusSkip
			} else if ruleResponse.Status() == engineapi.RuleStatusError {
				result.Result = policyreportv1alpha2.StatusError
			} else if ruleResponse.Status() == engineapi.RuleStatusPass {
				result.Result = policyreportv1alpha2.StatusPass
			} else if ruleResponse.Status() == engineapi.RuleStatusFail {
				if scored, ok := ann[kyvernov1.AnnotationPolicyScored]; ok && scored == "false" {
					result.Result = policyreportv1alpha2.StatusWarn
				} else if auditWarn && engineResponse.GetValidationFailureAction().Audit() {
					result.Result = policyreportv1alpha2.StatusWarn
				} else {
					result.Result = policyreportv1alpha2.StatusFail
				}
			} else {
				fmt.Println(ruleResponse)
			}

			result.Rule = ruleResponse.Name()
			result.Message = ruleResponse.Message()
			result.Source = kyvernov1.ValueKyvernoApp
			result.Timestamp = now
			results[appname] = append(results[appname], result)
		}
	}

	return results
}

func calculateSummary(results []policyreportv1alpha2.PolicyReportResult) (summary policyreportv1alpha2.PolicyReportSummary) {
	for _, res := range results {
		switch string(res.Result) {
		case policyreportv1alpha2.StatusPass:
			summary.Pass++
		case policyreportv1alpha2.StatusFail:
			summary.Fail++
		case "warn":
			summary.Warn++
		case "error":
			summary.Error++
		case "skip":
			summary.Skip++
		}
	}
	return
}
