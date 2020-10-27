package apply

import (
	"os"
	"testing"

	report "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var engineResponses = []response.EngineResponse{
	{
		PatchedResource: unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind": "Pod",
				"metadata": map[string]interface{}{
					"name":      "policy1-pod",
					"namespace": "policy1-namespace",
				},
			},
		},
		PolicyResponse: response.PolicyResponse{
			Policy:   "policy1",
			Resource: response.ResourceSpec{Name: "policy1-pod"},
			Rules: []response.RuleResponse{
				{
					Name:    "policy1-rule1",
					Success: true,
				},
				{
					Name:    "policy1-rule2",
					Success: false,
				},
			},
		},
	},
	{
		PatchedResource: unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind": "ClusterRole",
				"metadata": map[string]interface{}{
					"name": "policy2-clusterrole",
				},
			},
		},
		PolicyResponse: response.PolicyResponse{
			Policy:   "clusterpolicy2",
			Resource: response.ResourceSpec{Name: "policy2-clusterrole"},
			Rules: []response.RuleResponse{
				{
					Name:    "clusterpolicy2-rule1",
					Success: true,
				},
				{
					Name:    "clusterpolicy2-rule2",
					Success: false,
				},
			},
		},
	},
}

func Test_buildPolicyReports(t *testing.T) {
	os.Setenv("POLICY-TYPE", common.PolicyReport)
	reports := buildPolicyReports(engineResponses)
	assert.Assert(t, len(reports) == 2, len(reports))

	for _, report := range reports {
		if report.GetNamespace() == "" {
			assert.Assert(t, report.GetName() == clusterpolicyreport)
			assert.Assert(t, report.GetKind() == "ClusterPolicyReport")
			assert.Assert(t, len(report.UnstructuredContent()["results"].([]interface{})) == 2)
			assert.Assert(t,
				report.UnstructuredContent()["summary"].(map[string]interface{})["Pass"].(int64) == 1,
				report.UnstructuredContent()["summary"].(map[string]interface{})["Pass"].(int64))
		} else {
			assert.Assert(t, report.GetName() == "policyreport-ns-policy1-namespace")
			assert.Assert(t, report.GetKind() == "PolicyReport")
			assert.Assert(t, len(report.UnstructuredContent()["results"].([]interface{})) == 2)
			assert.Assert(t,
				report.UnstructuredContent()["summary"].(map[string]interface{})["Pass"].(int64) == 1,
				report.UnstructuredContent()["summary"].(map[string]interface{})["Pass"].(int64))
		}
	}
}

func Test_buildPolicyResults(t *testing.T) {
	os.Setenv("POLICY-TYPE", common.PolicyReport)

	results := buildPolicyResults(engineResponses)
	assert.Assert(t, len(results[clusterpolicyreport]) == 2, len(results[clusterpolicyreport]))
	assert.Assert(t, len(results["policyreport-ns-policy1-namespace"]) == 2, len(results["policyreport-ns-policy1-namespace"]))

	for _, result := range results {
		assert.Assert(t, len(result) == 2, len(result))
		for _, r := range result {
			switch r.Rule {
			case "policy1-rule1", "clusterpolicy2-rule1":
				assert.Assert(t, r.Status == report.PolicyStatus("Pass"))
			case "policy1-rule2", "clusterpolicy2-rule2":
				assert.Assert(t, r.Status == report.PolicyStatus("Fail"))
			}
		}
	}
}

func Test_mergeSucceededResults(t *testing.T) {
	resultsMap := map[string][]*report.PolicyReportResult{
		clusterpolicyreport: {
			{
				Status: report.PolicyStatus("Pass"),
				Policy: "clusterpolicy",
				Rule:   "clusterrule",
				Resources: []*v1.ObjectReference{
					{
						Name: "cluster-resource-1",
					},
					{
						Name: "cluster-resource-2",
					},
				},
			},
			{
				Status: report.PolicyStatus("Pass"),
				Policy: "clusterpolicy",
				Rule:   "clusterrule",
				Resources: []*v1.ObjectReference{
					{
						Name: "cluster-resource-3",
					},
					{
						Name: "cluster-resource-4",
					},
				},
			},
			{
				Status: report.PolicyStatus("Fail"),
				Policy: "clusterpolicy",
				Rule:   "clusterrule",
				Resources: []*v1.ObjectReference{
					{
						Name: "cluster-resource-5",
					},
				},
			},
		},

		"policyreport-ns-test1": {
			{
				Status: report.PolicyStatus("Pass"),
				Policy: "test1-policy",
				Rule:   "test1-rule",
				Resources: []*v1.ObjectReference{
					{
						Name: "resource-1",
					},
					{
						Name: "resource-2",
					},
				},
			},
			{
				Status: report.PolicyStatus("Pass"),
				Policy: "test1-policy",
				Rule:   "test1-rule",
				Resources: []*v1.ObjectReference{
					{
						Name: "resource-3",
					},
					{
						Name: "resource-4",
					},
				},
			},
			{
				Status: report.PolicyStatus("Fail"),
				Policy: "test1-policy",
				Rule:   "test1-rule",
				Resources: []*v1.ObjectReference{
					{
						Name: "resource-5",
					},
				},
			},
		},
		"policyreport-ns-test2": {
			{
				Status: report.PolicyStatus("Pass"),
				Policy: "test2-policy",
				Rule:   "test2-rule",
				Resources: []*v1.ObjectReference{
					{
						Name: "resource-1",
					},
					{
						Name: "resource-2",
					},
				},
			},
		},
	}

	results := mergeSucceededResults(resultsMap)
	assert.Assert(t, len(results) == len(resultsMap), len(results), len(resultsMap))
	for key, result := range results {
		if key == clusterpolicyreport {
			assert.Assert(t, len(result) == 3, len(result))
			for _, res := range result {
				if res.Status == report.PolicyStatus("Pass") {
					assert.Assert(t, len(res.Resources) == 2, len(res.Resources))
				} else {
					assert.Assert(t, len(res.Resources) == 1, len(res.Resources))
				}
			}
		}

		if key == "policyreport-ns-test1" {
			assert.Assert(t, len(result) == 3, len(result))
			for _, res := range result {
				if res.Status == report.PolicyStatus("Pass") {
					assert.Assert(t, len(res.Resources) == 2, len(res.Resources))
				} else {
					assert.Assert(t, len(res.Resources) == 1, len(res.Resources))
				}
			}
		}

		if key == "policyreport-ns-test2" {
			assert.Assert(t, len(result[0].Resources) == 2, len(result[0].Resources))
		}
	}
}

func Test_calculateSummary(t *testing.T) {
	results := []*report.PolicyReportResult{
		{
			Resources: make([]*v1.ObjectReference, 5),
			Status:    report.PolicyStatus("Pass"),
		},
		{Status: report.PolicyStatus("Fail")},
		{Status: report.PolicyStatus("Fail")},
		{Status: report.PolicyStatus("Fail")},
		{
			Resources: make([]*v1.ObjectReference, 1),
			Status:    report.PolicyStatus("Pass")},
		{
			Resources: make([]*v1.ObjectReference, 4),
			Status:    report.PolicyStatus("Pass"),
		},
	}

	summary := calculateSummary(results)
	assert.Assert(t, summary.Pass == 10)
	assert.Assert(t, summary.Fail == 3)
}
