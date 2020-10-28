package policyreport

import (
	"fmt"
	"reflect"

	"github.com/cornelk/hashmap"
	report "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type deletedResource struct {
	kind, ns, name string
}

func getDeletedResources(aggregatedRequests interface{}) (resources []deletedResource) {
	if requests, ok := aggregatedRequests.([]*report.ClusterReportRequest); ok {
		for _, request := range requests {
			var dr deletedResource
			if resource, ok := request.GetLabels()["delete"]; ok {
				dr.kind, dr.ns, dr.name = getDeletedResourceLabelValue(resource)
				resources = append(resources, dr)
			}
		}
	} else if requests, ok := aggregatedRequests.([]*report.ReportRequest); ok {
		for _, request := range requests {
			var dr deletedResource
			if resource, ok := request.GetLabels()["delete"]; ok {
				dr.kind, dr.ns, dr.name = getDeletedResourceLabelValue(resource)
				resources = append(resources, dr)
			}
		}
	}
	return
}

func updateResults(oldReport, newReport map[string]interface{}, aggregatedRequests interface{}) (map[string]interface{}, error) {
	deleteResources := getDeletedResources(aggregatedRequests)
	oldResults := hashResults(oldReport, deleteResources)

	if newresults, ok := newReport["results"].([]interface{}); ok {
		for _, res := range newresults {
			resMap, ok := res.(map[string]interface{})
			if !ok {
				continue
			}
			if key, ok := generateHashKey(resMap, deletedResource{}); ok {
				oldResults.Set(key, res)
			}
		}
	}

	results := getResultsFromHash(oldResults)
	if err := unstructured.SetNestedSlice(newReport, results, "results"); err != nil {
		return nil, err
	}

	summary := updateSummary(results)
	if err := unstructured.SetNestedMap(newReport, summary, "summary"); err != nil {
		return nil, err
	}
	return newReport, nil
}

func hashResults(policyReport map[string]interface{}, deleteResources []deletedResource) *hashmap.HashMap {
	resultsHash := &hashmap.HashMap{}

	results, ok := policyReport["results"]
	if !ok {
		return resultsHash
	}

	for _, result := range results.([]interface{}) {
		if len(deleteResources) != 0 {
			for _, dr := range deleteResources {
				if key, ok := generateHashKey(result.(map[string]interface{}), dr); ok {
					resultsHash.Set(key, result)
				}
			}
		} else {
			if key, ok := generateHashKey(result.(map[string]interface{}), deletedResource{}); ok {
				resultsHash.Set(key, result)
			}
		}
	}
	return resultsHash
}

func getResultsFromHash(resHash *hashmap.HashMap) []interface{} {
	results := make([]interface{}, 0)

	for result := range resHash.Iter() {
		if reflect.DeepEqual(result, hashmap.KeyValue{}) {
			continue
		}

		results = append(results, result.Value.(map[string]interface{}))

	}
	return results
}

func generateHashKey(result map[string]interface{}, dr deletedResource) (string, bool) {
	resources := result["resources"].([]interface{})
	if len(resources) < 1 {
		return "", false
	}

	resource := resources[0].(map[string]interface{})
	if !reflect.DeepEqual(dr, deletedResource{}) {
		if resource["kind"] == dr.kind && resource["name"] == dr.name && resource["namespace"] == dr.ns {
			return "", false
		}
	}

	return fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		result["policy"],
		result["rule"],
		resource["name"],
		resource["namespace"],
		resource["name"]), true
}

func updateSummary(results []interface{}) map[string]interface{} {
	summary := make(map[string]interface{}, 5)

	for _, result := range results {
		typedResult, ok := result.(map[string]interface{})
		if !ok {
			continue
		}

		switch typedResult["status"].(string) {
		case report.StatusPass:
			pass, _ := summary["Pass"].(int64)
			summary["Pass"] = pass + 1
		case report.StatusFail:
			fail, _ := summary["Fail"].(int64)
			summary["Fail"] = fail + 1
		case "Warn":
			warn, _ := summary["Warn"].(int64)
			summary["warn"] = warn + 1
		case "Error":
			e, _ := summary["Error"].(int64)
			summary["Error"] = e + 1
		case "Skip":
			skip, _ := summary["Skip"].(int64)
			summary["Skip"] = skip + 1
		}
	}

	return summary
}
