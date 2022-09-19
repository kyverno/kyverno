package policyreport

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/cornelk/hashmap"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha2"
	policyreportv1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

type PolicyReportEraser interface {
	CleanupReportChangeRequests(cleanup CleanupReportChangeRequests, labels map[string]string) error
	EraseResultEntries(erase EraseResultEntries, ns *string) error
}

type (
	CleanupReportChangeRequests = func(pclient versioned.Interface, rcrLister kyvernov1alpha2listers.ReportChangeRequestLister, crcrLister kyvernov1alpha2listers.ClusterReportChangeRequestLister, labels map[string]string) error
	EraseResultEntries          = func(pclient versioned.Interface, reportLister policyreportv1alpha2listers.PolicyReportLister, clusterReportLister policyreportv1alpha2listers.ClusterPolicyReportLister, ns *string) error
)

func (g *ReportGenerator) CleanupReportChangeRequests(cleanup CleanupReportChangeRequests, labels map[string]string) error {
	return cleanup(g.pclient, g.reportChangeRequestLister, g.clusterReportChangeRequestLister, labels)
}

func (g *ReportGenerator) EraseResultEntries(erase EraseResultEntries, ns *string) error {
	return erase(g.pclient, g.reportLister, g.clusterReportLister, ns)
}

type deletedResource struct {
	kind, ns, name string
}

func buildLabelForDeletedResource(labels, annotations map[string]string) *deletedResource {
	ok := true
	kind, kindOk := annotations[deletedAnnotationResourceKind]
	ok = ok && kindOk

	name, nameOk := annotations[deletedAnnotationResourceName]
	ok = ok && nameOk

	if !ok {
		return nil
	}

	return &deletedResource{
		kind: kind,
		name: name,
		ns:   labels[ResourceLabelNamespace],
	}
}

func getDeletedResources(aggregatedRequests interface{}) (resources []deletedResource) {
	if requests, ok := aggregatedRequests.([]*kyvernov1alpha2.ClusterReportChangeRequest); ok {
		for _, request := range requests {
			dr := buildLabelForDeletedResource(request.GetLabels(), request.GetAnnotations())
			if dr != nil {
				resources = append(resources, *dr)
			}
		}
	} else if requests, ok := aggregatedRequests.([]*kyvernov1alpha2.ReportChangeRequest); ok {
		for _, request := range requests {
			dr := buildLabelForDeletedResource(request.GetLabels(), request.GetAnnotations())
			if dr != nil {
				resources = append(resources, *dr)
			}
		}
	}
	return
}

func updateResults(oldReport, newReport map[string]interface{}, aggregatedRequests interface{}) (map[string]interface{}, bool, error) {
	deleteResources := getDeletedResources(aggregatedRequests)
	oldResults := hashResults(oldReport, deleteResources)
	var hasDuplicate bool

	if newresults, ok := newReport["results"].([]interface{}); ok {
		for _, res := range newresults {
			resMap, ok := res.(map[string]interface{})
			if !ok {
				continue
			}
			if key, ok := generateHashKey(resMap, deletedResource{}); ok {
				if _, exist := oldResults.Get(key); exist {
					hasDuplicate = exist
				}

				oldResults.Set(key, res)
			}
		}
	}

	results := getResultsFromHash(oldResults)
	if err := unstructured.SetNestedSlice(newReport, results, "results"); err != nil {
		return nil, hasDuplicate, err
	}

	summaryResults := []policyreportv1alpha2.PolicyReportResult{}
	if err := mapToStruct(results, &summaryResults); err != nil {
		return nil, hasDuplicate, err
	}

	summary := updateSummary(summaryResults)
	if err := unstructured.SetNestedMap(newReport, summary.ToMap(), "summary"); err != nil {
		return nil, hasDuplicate, err
	}
	return newReport, hasDuplicate, nil
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
		if resource["kind"] == dr.kind {
			if resource["name"] == dr.name && resource["namespace"] == dr.ns {
				return "", false
			}

			if dr.kind == "Namespace" && resource["name"] == dr.name {
				return "", false
			}
		}
	}

	return fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		result["policy"],
		result["rule"],
		resource["kind"],
		resource["namespace"],
		resource["name"]), true
}

func updateSummary(results []policyreportv1alpha2.PolicyReportResult) policyreportv1alpha2.PolicyReportSummary {
	summary := policyreportv1alpha2.PolicyReportSummary{}

	for _, result := range results {
		switch result.Result {
		case policyreportv1alpha2.StatusPass:
			summary.Pass++
		case policyreportv1alpha2.StatusFail:
			summary.Fail++
		case policyreportv1alpha2.StatusWarn:
			summary.Warn++
		case policyreportv1alpha2.StatusError:
			summary.Error++
		case policyreportv1alpha2.StatusSkip:
			summary.Skip++
		}
	}

	return summary
}

func isDeletedPolicyKey(key string) (policyName, ruleName string, isDelete bool) {
	policy := strings.Split(key, "/")

	if policy[0] == deletedPolicyKey {
		// deletedPolicyKey/policyName/ruleName
		if len(policy) == 3 {
			return policy[1], policy[2], true
		}
		// deletedPolicyKey/policyName
		if len(policy) == 2 {
			return policy[1], "", true
		}
	}

	return "", "", false
}

func mapToStruct(in, out interface{}) error {
	jsonBytes, _ := json.Marshal(in)
	return json.Unmarshal(jsonBytes, out)
}

func CleanupPolicyReport(client versioned.Interface) error {
	var errors []string
	var gracePeriod int64 = 0

	deleteOptions := metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}
	selector := labels.SelectorFromSet(labels.Set(map[string]string{LabelSelectorKey: kyvernov1.ValueKyvernoApp}))

	err := client.KyvernoV1alpha2().ClusterReportChangeRequests().DeleteCollection(context.TODO(), deleteOptions, metav1.ListOptions{})
	if err != nil {
		errors = append(errors, err.Error())
	}

	err = client.KyvernoV1alpha2().ReportChangeRequests(config.KyvernoNamespace()).DeleteCollection(context.TODO(), deleteOptions, metav1.ListOptions{})
	if err != nil {
		errors = append(errors, err.Error())
	}

	err = client.Wgpolicyk8sV1alpha2().ClusterPolicyReports().DeleteCollection(context.TODO(), deleteOptions, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		errors = append(errors, err.Error())
	}

	reports, err := client.Wgpolicyk8sV1alpha2().PolicyReports(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		errors = append(errors, err.Error())
	}
	for _, report := range reports.Items {
		err = client.Wgpolicyk8sV1alpha2().PolicyReports(report.Namespace).Delete(context.TODO(), report.Name, metav1.DeleteOptions{})
		if err != nil {
			errors = append(errors, err.Error())
		}
	}
	if len(errors) == 0 {
		return nil
	}
	return fmt.Errorf("%v", strings.Join(errors, ";"))
}
