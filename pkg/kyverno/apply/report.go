package apply

import (
	"encoding/json"
	"fmt"
	"strings"

	report "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/policyreport"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

const clusterpolicyreport = "clusterpolicyreport"

// resps is the engine responses generated for a single policy
func buildPolicyReports(resps []response.EngineResponse) (res []*unstructured.Unstructured) {
	var raw []byte
	var err error

	resultsMap := buildPolicyResults(resps)
	for scope, result := range resultsMap {
		if scope == clusterpolicyreport {
			report := &report.ClusterPolicyReport{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "policy.k8s.io/v1alpha1",
					Kind:       "ClusterPolicyReport",
				},
				Results: result,
				Summary: calculateSummary(result),
			}

			report.SetName(scope)
			if raw, err = json.Marshal(report); err != nil {
				log.Log.Error(err, "failed to serilize policy report", "name", report.Name, "scope", scope)
			}
		} else {
			report := &report.PolicyReport{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "policy.k8s.io/v1alpha1",
					Kind:       "PolicyReport",
				},
				Results: result,
				Summary: calculateSummary(result),
			}

			ns := strings.ReplaceAll(scope, "policyreport-ns-", "")
			report.SetName(scope)
			report.SetNamespace(ns)

			if raw, err = json.Marshal(report); err != nil {
				log.Log.Error(err, "failed to serilize policy report", "name", report.Name, "scope", scope)
			}
		}

		reportUnstructured, err := engineutils.ConvertToUnstructured(raw)
		if err != nil {
			log.Log.Error(err, "failed to convert policy report", "scope", scope)
			continue
		}

		res = append(res, reportUnstructured)
	}

	return
}

// buildPolicyResults returns a string-PolicyReportResult map
// the key of the map is one of "clusterpolicyreport", "policyreport-ns-<namespace>"
func buildPolicyResults(resps []response.EngineResponse) map[string][]*report.PolicyReportResult {
	results := make(map[string][]*report.PolicyReportResult)
	infos := policyreport.GeneratePRsFromEngineResponse(resps, log.Log)

	for _, info := range infos {
		var appname string

		ns := info.Resource.GetNamespace()
		if ns != "" {
			appname = fmt.Sprintf("policyreport-ns-%s", ns)
		} else {
			appname = fmt.Sprintf(clusterpolicyreport)
		}

		for _, rule := range info.Rules {
			result := report.PolicyReportResult{
				Policy: info.PolicyName,
				Resources: []*corev1.ObjectReference{
					{
						Kind:       info.Resource.GetKind(),
						Namespace:  info.Resource.GetNamespace(),
						APIVersion: info.Resource.GetAPIVersion(),
						Name:       info.Resource.GetName(),
						UID:        info.Resource.GetUID(),
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

	return mergeSucceededResults(results)
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
			summary.Pass += len(res.Resources)
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
