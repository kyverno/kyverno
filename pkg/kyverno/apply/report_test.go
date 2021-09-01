package apply

import (
	"testing"

	preport "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha2"
	report "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha2"
	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
)

// var engineResponses = []*response.EngineResponse{
// 	{
// 		PatchedResource: unstructured.Unstructured{
// 			Object: map[string]interface{}{
// 				"kind": "Pod",
// 				"metadata": map[string]interface{}{
// 					"name":      "policy1-pod",
// 					"namespace": "policy1-namespace",
// 				},
// 			},
// 		},
// 		PolicyResponse: response.PolicyResponse{
// 			Policy:   response.PolicySpec{Name: "policy1"},
// 			Resource: response.ResourceSpec{Name: "policy1-pod"},
// 			Rules: []response.RuleResponse{
// 				{
// 					Name:    "policy1-rule1",
// 					Type:    utils.Validation.String(),
// 					Success: true,
// 				},
// 				{
// 					Name:    "policy1-rule2",
// 					Type:    utils.Validation.String(),
// 					Success: false,
// 				},
// 			},
// 		},
// 	},
// 	{
// 		PatchedResource: unstructured.Unstructured{
// 			Object: map[string]interface{}{
// 				"kind": "ClusterRole",
// 				"metadata": map[string]interface{}{
// 					"name": "policy2-clusterrole",
// 				},
// 			},
// 		},
// 		PolicyResponse: response.PolicyResponse{
// 			Policy:   response.PolicySpec{Name: "clusterpolicy2"},
// 			Resource: response.ResourceSpec{Name: "policy2-clusterrole"},
// 			Rules: []response.RuleResponse{
// 				{
// 					Name:    "clusterpolicy2-rule1",
// 					Type:    utils.Validation.String(),
// 					Success: true,
// 				},
// 				{
// 					Name:    "clusterpolicy2-rule2",
// 					Type:    utils.Validation.String(),
// 					Success: false,
// 				},
// 			},
// 		},
// 	},
// }

// func Test_buildPolicyReports(t *testing.T) {
// 	os.Setenv("POLICY-TYPE", common.PolicyReport)
// 	reports := buildPolicyReports(engineResponses, nil)
// 	assert.Assert(t, len(reports) == 2, len(reports))

// 	for _, report := range reports {
// 		if report.GetNamespace() == "" {
// 			assert.Assert(t, report.GetName() == clusterpolicyreport)
// 			assert.Assert(t, report.GetKind() == "ClusterPolicyReport")
// 			assert.Assert(t, len(report.UnstructuredContent()["results"].([]interface{})) == 2)
// 			assert.Assert(t,
// 				report.UnstructuredContent()["summary"].(map[string]interface{})[preport.StatusPass].(int64) == 1,
// 				report.UnstructuredContent()["summary"].(map[string]interface{})[preport.StatusPass].(int64))
// 		} else {
// 			assert.Assert(t, report.GetName() == "policyreport-ns-policy1-namespace")
// 			assert.Assert(t, report.GetKind() == "PolicyReport")
// 			assert.Assert(t, len(report.UnstructuredContent()["results"].([]interface{})) == 2)
// 			assert.Assert(t,
// 				report.UnstructuredContent()["summary"].(map[string]interface{})[preport.StatusPass].(int64) == 1,
// 				report.UnstructuredContent()["summary"].(map[string]interface{})[preport.StatusPass].(int64))
// 		}
// 	}
// }

// func Test_buildPolicyResults(t *testing.T) {
// 	os.Setenv("POLICY-TYPE", common.PolicyReport)

// 	results := buildPolicyResults(engineResponses, nil)
// 	assert.Assert(t, len(results[clusterpolicyreport]) == 2, len(results[clusterpolicyreport]))
// 	assert.Assert(t, len(results["policyreport-ns-policy1-namespace"]) == 2, len(results["policyreport-ns-policy1-namespace"]))

// 	for _, result := range results {
// 		assert.Assert(t, len(result) == 2, len(result))
// 		for _, r := range result {
// 			switch r.Rule {
// 			case "policy1-rule1", "clusterpolicy2-rule1":
// 				assert.Assert(t, r.Result == report.PolicyResult(preport.StatusPass))
// 			case "policy1-rule2", "clusterpolicy2-rule2":
// 				assert.Assert(t, r.Result == report.PolicyResult(preport.StatusFail))
// 			}
// 		}
// 	}
// }

func Test_calculateSummary(t *testing.T) {
	results := []*report.PolicyReportResult{
		{
			Resources: make([]*v1.ObjectReference, 5),
			Result:    report.PolicyResult(preport.StatusPass),
		},
		{Result: report.PolicyResult(preport.StatusFail)},
		{Result: report.PolicyResult(preport.StatusFail)},
		{Result: report.PolicyResult(preport.StatusFail)},
		{
			Resources: make([]*v1.ObjectReference, 1),
			Result:    report.PolicyResult(preport.StatusPass)},
		{
			Resources: make([]*v1.ObjectReference, 4),
			Result:    report.PolicyResult(preport.StatusPass),
		},
	}

	summary := calculateSummary(results)
	assert.Assert(t, summary.Pass == 3)
	assert.Assert(t, summary.Fail == 3)
}
