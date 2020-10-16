package apply

import (
	"encoding/json"
	"fmt"
	"strings"

	policyreportv1alpha1 "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/policyreport"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

const clusterpolicyreport = "clusterpolicyreport"

// resps is the engine reponses generated for a single policy
func buildPolicyReports(resps []response.EngineResponse) (res []*unstructured.Unstructured) {
	var raw []byte
	var err error

	resultsMap := buildPolicyResults(resps)
	for scope, result := range resultsMap {
		if scope == clusterpolicyreport {
			report := &policyreportv1alpha1.ClusterPolicyReport{
				Results: result,
				Summary: calculateSummary(result),
			}

			report.SetName(scope)
			if raw, err = json.Marshal(report); err != nil {
				log.Log.Error(err, "failed to serilize policy report", "name", report.Name, "scope", scope)
			}
		} else {
			report := &policyreportv1alpha1.PolicyReport{
				Results: result,
				Summary: calculateSummary(result),
			}

			ns := strings.ReplaceAll(scope, "policyreport-ns-", "")
			report.SetName(scope)
			report.SetNamespace(ns)
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
func buildPolicyResults(resps []response.EngineResponse) map[string][]*policyreportv1alpha1.PolicyReportResult {
	results := make(map[string][]*policyreportv1alpha1.PolicyReportResult)
	infos := policyreport.GeneratePRsFromEngineResponse(resps, log.Log)

	for _, info := range infos {
		var appname string

		ns := info.Resource.GetNamespace()
		if ns != "" {
			appname = fmt.Sprintf("policyreport-ns-%s", ns)
		} else {
			appname = fmt.Sprintf(clusterpolicyreport)
		}

		result := &policyreportv1alpha1.PolicyReportResult{
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
		}

		for _, rule := range info.Rules {
			result.Rule = rule.Name
			result.Message = rule.Message
			result.Status = policyreportv1alpha1.PolicyStatus(rule.Check)
			results[appname] = append(results[appname], result)
		}
	}

	return mergeSucceededResults(results)
}

func mergeSucceededResults(results map[string][]*policyreportv1alpha1.PolicyReportResult) map[string][]*policyreportv1alpha1.PolicyReportResult {
	resultsNew := make(map[string][]*policyreportv1alpha1.PolicyReportResult)

	for scope, scopedResults := range results {

		resourcesMap := make(map[string]*policyreportv1alpha1.PolicyReportResult)
		for _, result := range scopedResults {
			if result.Status != policyreportv1alpha1.PolicyStatus("Pass") {
				resultsNew[scope] = append(resultsNew[scope], result)
				continue
			}

			key := fmt.Sprintf("%s/%s", result.Policy, result.Rule)
			if r, ok := resourcesMap[key]; !ok {
				resourcesMap[key] = &policyreportv1alpha1.PolicyReportResult{}
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

			r := &policyreportv1alpha1.PolicyReportResult{
				Policy:    names[0],
				Rule:      names[1],
				Resources: v.Resources,
				Status:    policyreportv1alpha1.PolicyStatus("Pass"),
				Scored:    true,
			}

			resultsNew[scope] = append(resultsNew[scope], r)
		}
	}
	return resultsNew
}

func calculateSummary(results []*policyreportv1alpha1.PolicyReportResult) (summary policyreportv1alpha1.PolicyReportSummary) {
	for _, res := range results {
		switch string(res.Status) {
		case "Pass":
			summary.Pass++
		case "Fail":
			summary.Fail++
		case "Warn":
			summary.Warn++
		case "Error":
			summary.Error++
		case "Skip":
			summary.Skip++
		}
	}
	return
}
