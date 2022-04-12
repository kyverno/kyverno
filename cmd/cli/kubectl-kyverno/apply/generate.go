package apply

import (
	"reflect"

	report "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	client "github.com/kyverno/kyverno/pkg/dclient"
	sanitizederror "github.com/kyverno/kyverno/pkg/kyverno/sanitizedError"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// generateToCluster updates the existing policy reports in the cluster
// creates new report if not exist
func generateToCluster(dClient *client.Client, reports []*unstructured.Unstructured) {
	var clusterReports, namespaceReports []*unstructured.Unstructured
	for _, report := range reports {
		if report.GetNamespace() == "" {
			clusterReports = append(clusterReports, report)
		} else {
			namespaceReports = append(namespaceReports, report)
		}
	}

	if clusterReport, err := mergeClusterReport(clusterReports); err != nil {
		log.Log.V(3).Info("failed to merge cluster report", "error", err)
	} else {
		if err := updateReport(dClient, clusterReport); err != nil {
			log.Log.V(3).Info("failed to update policy report", "report", clusterReport.GetName(), "error", err)
		}
	}

	for _, report := range namespaceReports {
		if err := updateReport(dClient, report); err != nil {
			log.Log.V(3).Info("failed to update policy report", "report", report.GetName(), "error", err)
		}
	}
}

func updateReport(dClient *client.Client, new *unstructured.Unstructured) error {
	old, err := dClient.GetResource(new.GetAPIVersion(), new.GetKind(), new.GetNamespace(), new.GetName())
	if err != nil {
		if apierrors.IsNotFound(err) {
			if _, err := dClient.CreateResource(new.GetAPIVersion(), new.GetKind(), new.GetNamespace(), new, false); err != nil {
				return err
			}
		}
		return err
	}

	oldResults, _, err := unstructured.NestedSlice(old.UnstructuredContent(), "results")
	if err != nil {
		log.Log.V(3).Info("failed to get results entry", "error", err)
	}

	newResults, _, err := unstructured.NestedSlice(new.UnstructuredContent(), "results")
	if err != nil {
		log.Log.V(3).Info("failed to get results entry", "error", err)
	}

	if reflect.DeepEqual(oldResults, newResults) {
		log.Log.V(3).Info("policy report unchanged", "name", new.GetName())
		return nil
	}

	_, err = dClient.UpdateResource(new.GetAPIVersion(), new.GetKind(), new.GetNamespace(), new, false)
	return err
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
