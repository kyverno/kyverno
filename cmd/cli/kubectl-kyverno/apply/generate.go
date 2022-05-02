package apply

import (
	report "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// generateCLIRaw merges all policy reports to a singe cluster policy report
func generateCLIRaw(reports []*unstructured.Unstructured) (*unstructured.Unstructured, error) {
	for _, report := range reports {
		if report.GetNamespace() != "" {
			report.SetNamespace("")
		}
	}

	return mergeClusterReport(reports)
}

func mergeClusterReport(reports []*unstructured.Unstructured) (*unstructured.Unstructured, error) {
	var resultsEntry []interface{}
	res := &unstructured.Unstructured{}
	res.SetName(clusterpolicyreport)
	res.SetKind("ClusterPolicyReport")
	res.SetAPIVersion(report.SchemeGroupVersion.String())

	for _, report := range reports {
		if report.GetNamespace() != "" {
			// skip namespace report
			continue
		}

		mergeResults(report, &resultsEntry)
	}

	if err := unstructured.SetNestedSlice(res.Object, resultsEntry, "results"); err != nil {
		return nil, sanitizederror.NewWithError("failed to set results entry", err)
	}

	summary := updateSummary(resultsEntry)
	if err := unstructured.SetNestedField(res.Object, summary, "summary"); err != nil {
		return nil, sanitizederror.NewWithError("failed to set summary", err)
	}

	return res, nil
}

func mergeResults(report *unstructured.Unstructured, results *[]interface{}) {
	entries, ok, err := unstructured.NestedSlice(report.UnstructuredContent(), "results")
	if err != nil {
		log.Log.V(3).Info("failed to get results entry", "report", report.GetName(), "error", err)
	}

	if ok {
		*results = append(*results, entries...)
	}
}

func updateSummary(results []interface{}) map[string]interface{} {
	summary := make(map[string]interface{})
	status := []string{report.StatusPass, report.StatusFail, report.StatusError, report.StatusSkip, report.StatusWarn}
	for i := 0; i < 5; i++ {
		if _, ok := summary[status[i]].(int64); !ok {
			summary[status[i]] = int64(0)
		}
	}
	for _, result := range results {
		typedResult, ok := result.(map[string]interface{})
		if !ok {
			continue
		}

		switch typedResult["result"].(string) {
		case report.StatusPass:
			pass, _ := summary[report.StatusPass].(int64)
			pass++
			summary[report.StatusPass] = pass
		case report.StatusFail:
			fail, _ := summary[report.StatusFail].(int64)
			fail++
			summary[report.StatusFail] = fail
		case report.StatusWarn:
			warn, _ := summary[report.StatusWarn].(int64)
			warn++
			summary[report.StatusWarn] = warn
		case report.StatusError:
			e, _ := summary[report.StatusError].(int64)
			e++
			summary[report.StatusError] = e
		case report.StatusSkip:
			skip, _ := summary[report.StatusSkip].(int64)
			skip++
			summary[report.StatusSkip] = skip
		}
	}

	return summary
}
