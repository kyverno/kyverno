package apply

import (
	"reflect"

	report "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/kyverno/sanitizedError"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// generateCLIraw merges all policy reports to a singe cluster policy report
func generateCLIraw(reports []*unstructured.Unstructured) (*unstructured.Unstructured, error) {
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
		log.Log.Error(err, "failed to merge cluster report")
	} else {
		if err := updateReport(dClient, clusterReport); err != nil {
			log.Log.Error(err, "failed to update policy report", "report", clusterReport.GetName())
		}
	}

	for _, report := range namespaceReports {
		if err := updateReport(dClient, report); err != nil {
			log.Log.Error(err, "failed to update policy report", "report", report.GetName())
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
		log.Log.Error(err, "failed to get results entry")
	}

	newResults, _, err := unstructured.NestedSlice(new.UnstructuredContent(), "results")
	if err != nil {
		log.Log.Error(err, "failed to get results entry")
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
	res.SetAPIVersion("policy.k8s.io/v1alpha1")

	for _, report := range reports {
		if report.GetNamespace() != "" {
			// skip namespace report
			continue
		}

		mergeResults(report, &resultsEntry)
	}

	if err := unstructured.SetNestedSlice(res.Object, resultsEntry, "results"); err != nil {
		return nil, sanitizedError.NewWithError("failed to set results entry", err)
	}

	summary := updateSummary(resultsEntry)
	if err := unstructured.SetNestedField(res.Object, summary, "summary"); err != nil {
		return nil, sanitizedError.NewWithError("failed to set summary", err)
	}

	return res, nil
}

func mergeResults(report *unstructured.Unstructured, results *[]interface{}) {
	entries, ok, err := unstructured.NestedSlice(report.UnstructuredContent(), "results")
	if err != nil {
		log.Log.Error(err, "failed to get results entry", "report", report.GetName())
	}

	if ok {
		*results = append(*results, entries...)
	}
}

func updateSummary(results []interface{}) map[string]interface{} {
	summary := make(map[string]interface{})

	for _, result := range results {
		typedResult, ok := result.(map[string]interface{})
		if !ok {
			continue
		}

		switch typedResult["status"].(string) {
		case report.StatusPass:
			//resources, ok := typedResult["resources"].([]interface{})
			//if !ok {
			//	continue
			//}

			pass, _ := summary["Pass"].(int64)
			pass++
			summary["Pass"] = pass
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
