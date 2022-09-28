package apply

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/engine/response"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/policyreport"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

const clusterpolicyreport = "clusterpolicyreport"

// resps is the engine responses generated for a single policy
func buildPolicyReports(pvInfos []policyreport.Info) (res []*unstructured.Unstructured) {
	var raw []byte
	var err error

	resultsMap := buildPolicyResults(pvInfos)
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

		reportUnstructured, err := engineutils.ConvertToUnstructured(raw)
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
func buildPolicyResults(infos []policyreport.Info) map[string][]policyreportv1alpha2.PolicyReportResult {
	results := make(map[string][]policyreportv1alpha2.PolicyReportResult)
	now := metav1.Timestamp{Seconds: time.Now().Unix()}

	for _, info := range infos {
		var appname string
		ns := info.Namespace
		if ns != "" {
			appname = fmt.Sprintf("policyreport-ns-%s", ns)
		} else {
			appname = clusterpolicyreport
		}

		for _, infoResult := range info.Results {
			for _, rule := range infoResult.Rules {
				if rule.Type != string(response.Validation) {
					continue
				}

				result := policyreportv1alpha2.PolicyReportResult{
					Policy: info.PolicyName,
					Resources: []corev1.ObjectReference{
						{
							Kind:       infoResult.Resource.Kind,
							Namespace:  infoResult.Resource.Namespace,
							APIVersion: infoResult.Resource.APIVersion,
							Name:       infoResult.Resource.Name,
							UID:        types.UID(infoResult.Resource.UID),
						},
					},
					Scored: true,
				}

				result.Rule = rule.Name
				result.Message = rule.Message
				result.Result = policyreportv1alpha2.PolicyResult(rule.Status)
				result.Source = kyvernov1.ValueKyvernoApp
				result.Timestamp = now
				results[appname] = append(results[appname], result)
			}
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
