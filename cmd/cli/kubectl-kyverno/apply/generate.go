package apply

import (
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
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
	res.SetAPIVersion(policyreportv1alpha2.SchemeGroupVersion.String())

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
	status := []string{policyreportv1alpha2.StatusPass, policyreportv1alpha2.StatusFail, policyreportv1alpha2.StatusError, policyreportv1alpha2.StatusSkip, policyreportv1alpha2.StatusWarn}
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
		case policyreportv1alpha2.StatusPass:
			pass, _ := summary[policyreportv1alpha2.StatusPass].(int64)
			pass++
			summary[policyreportv1alpha2.StatusPass] = pass
		case policyreportv1alpha2.StatusFail:
			fail, _ := summary[policyreportv1alpha2.StatusFail].(int64)
			fail++
			summary[policyreportv1alpha2.StatusFail] = fail
		case policyreportv1alpha2.StatusWarn:
			warn, _ := summary[policyreportv1alpha2.StatusWarn].(int64)
			warn++
			summary[policyreportv1alpha2.StatusWarn] = warn
		case policyreportv1alpha2.StatusError:
			e, _ := summary[policyreportv1alpha2.StatusError].(int64)
			e++
			summary[policyreportv1alpha2.StatusError] = e
		case policyreportv1alpha2.StatusSkip:
			skip, _ := summary[policyreportv1alpha2.StatusSkip].(int64)
			skip++
			summary[policyreportv1alpha2.StatusSkip] = skip
		}
	}

	return summary
}
