package policyreport

import (
	"fmt"
	"reflect"

	"github.com/cornelk/hashmap"
	report "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func updateResults(oldReport, newReport map[string]interface{}) (map[string]interface{}, error) {
	oldResults := hashResults(oldReport)
	newresults := newReport["results"].([]interface{})

	for _, res := range newresults {
		resMap := res.(map[string]interface{})
		if key, ok := generateHashKey(resMap); ok {
			oldResults.Set(key, res)
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

func hashResults(policyReport map[string]interface{}) *hashmap.HashMap {
	resultsHash := &hashmap.HashMap{}

	results, ok := policyReport["results"]
	if !ok {
		return resultsHash
	}

	for _, result := range results.([]interface{}) {
		if key, ok := generateHashKey(result.(map[string]interface{})); ok {
			resultsHash.Set(key, result)
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

func generateHashKey(result map[string]interface{}) (string, bool) {
	resources := result["resources"].([]interface{})
	if len(resources) < 1 {
		return "", false
	}

	resource := resources[0].(map[string]interface{})
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
