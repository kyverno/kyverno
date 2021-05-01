package apply

import (
	"encoding/json"
	"fmt"
	"strings"

	report "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
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
func buildPolicyReports(resps []*response.EngineResponse, skippedPolicies []SkippedPolicy) (res []*unstructured.Unstructured) {
	var raw []byte
	var err error

	for _, sp := range skippedPolicies {
		for _, r := range sp.Rules {
			result := []*report.PolicyReportResult{
				{
					Message: fmt.Sprintln("skipped policy with variables -", sp.Variable),
					Policy:  sp.Name,
					Rule:    r.Name,
					Status:  "skip",
				},
			}

			report := &report.PolicyReport{
				TypeMeta: metav1.TypeMeta{
					APIVersion: report.SchemeGroupVersion.String(),
					Kind:       "PolicyReport",
				},
				Results: result,
			}

			if raw, err = json.Marshal(report); err != nil {
				log.Log.V(3).Info("failed to serialize policy report", "error", err)
				continue
			}

			reportUnstructured, err := engineutils.ConvertToUnstructured(raw)
			if err != nil {
				log.Log.V(3).Info("failed to convert policy report", "error", err)
				continue
			}

			res = append(res, reportUnstructured)
		}
	}

	resultsMap := buildPolicyResults(resps)
	for scope, result := range resultsMap {
		if scope == clusterpolicyreport {
			report := &report.ClusterPolicyReport{
				TypeMeta: metav1.TypeMeta{
					APIVersion: report.SchemeGroupVersion.String(),
					Kind:       "ClusterPolicyReport",
				},
				Results: generateSingleResult(result),
				Summary: calculateSummary(result),
			}

			report.SetName(scope)
			if raw, err = json.Marshal(report); err != nil {
				log.Log.V(3).Info("failed to serialize policy report", "name", report.Name, "scope", scope, "error", err)
			}
		} else {
			report := &report.PolicyReport{
				TypeMeta: metav1.TypeMeta{
					APIVersion: report.SchemeGroupVersion.String(),
					Kind:       "PolicyReport",
				},
				Results: generateSingleResult(result),
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
func buildPolicyResults(resps []*response.EngineResponse) map[string][]*report.PolicyReportResult {
	results := make(map[string][]*report.PolicyReportResult)
	infos := policyreport.GeneratePRsFromEngineResponse(resps, log.Log)

	for _, info := range infos {
		var appname string
		ns := info.Namespace
		if ns != "" {
			appname = fmt.Sprintf("policyreport-ns-%s", ns)
		} else {
			appname = fmt.Sprintf(clusterpolicyreport)
		}

		for _, infoResult := range info.Results {
			for _, rule := range infoResult.Rules {
				if rule.Type != utils.Validation.String() {
					continue
				}

				result := report.PolicyReportResult{
					Policy: info.PolicyName,
					Resources: []*corev1.ObjectReference{
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
				result.Status = report.PolicyStatus(rule.Check)
				results[appname] = append(results[appname], &result)
			}
		}
	}

	return results
}

func mergeSucceededResults(results map[string][]*report.PolicyReportResult) map[string][]*report.PolicyReportResult {
	resultsNew := make(map[string][]*report.PolicyReportResult)

	for scope, scopedResults := range results {

		resourcesMap := make(map[string]*report.PolicyReportResult)
		for _, result := range scopedResults {
			if result.Status != report.PolicyStatus("pass") {
				resultsNew[scope] = append(resultsNew[scope], result)
				continue
			}

			key := fmt.Sprintf("%s/%s", result.Policy, result.Rule)
			if r, ok := resourcesMap[key]; !ok {
				resourcesMap[key] = &report.PolicyReportResult{}
				resourcesMap[key] = result
			} else {
				r.Resources = append(r.Resources, result.Resources...)
				resourcesMap[key] = r
			}
		}

		for k, v := range resourcesMap {
			names := strings.Split(k, "/")
			if len(names) != 2 {
				continue
			}

			r := &report.PolicyReportResult{
				Policy:    names[0],
				Rule:      names[1],
				Resources: v.Resources,
				Status:    report.PolicyStatus(v.Status),
			}

			resultsNew[scope] = append(resultsNew[scope], r)
		}
	}
	return resultsNew
}

func calculateSummary(results []*report.PolicyReportResult) (summary report.PolicyReportSummary) {
	for _, res := range results {
		switch string(res.Status) {
		case report.StatusPass:
			summary.Pass++
		case report.StatusFail:
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

func generateSingleResult(results []*report.PolicyReportResult) []*report.PolicyReportResult {
	if len(results) <= 1 {
		return results
	}

	singleResult := []*report.PolicyReportResult{}

	for _, res := range results {
		if res.Status == report.StatusPass {
			if len(singleResult) == 0 {
				singleResult = append(singleResult, res)
			} else {
				singleResult[0].Resources = append(singleResult[0].Resources, res.Resources...)
			}
		}
	}

	return singleResult
}
