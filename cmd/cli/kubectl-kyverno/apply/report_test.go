package apply

import (
	"encoding/json"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	preport "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/engine/response"
	kyvCommon "github.com/kyverno/kyverno/pkg/kyverno/common"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
)

var rawPolicy = []byte(`
{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
	  "name": "pod-requirements",
	  "annotations": {
		"pod-policies.kyverno.io/autogen-controllers": "none"
	  }
	},
	"spec": {
	  "background": false,
	  "validationFailureAction": "audit",
	  "rules": [
		{
		  "name": "pods-require-account",
		  "match": {
			"resources": {
			  "kinds": [
				"Pod"
			  ]
			}
		  },
		  "validate": {
			"message": "User pods must include an account for charging",
			"pattern": {
			  "metadata": {
				"labels": {
				  "account": "*?"
				}
			  }
			}
		  }
		},
		{
		  "name": "pods-require-limits",
		  "match": {
			"resources": {
			  "kinds": [
				"Pod"
			  ]
			}
		  },
		  "validate": {
			"message": "CPU and memory resource requests and limits are required for user pods",
			"pattern": {
			  "spec": {
				"containers": [
				  {
					"resources": {
					  "requests": {
						"memory": "?*",
						"cpu": "?*"
					  },
					  "limits": {
						"memory": "?*",
						"cpu": "?*"
					  }
					}
				  }
				]
			  }
			}
		  }
		}
	  ]
	}
  }
`)

var rawEngRes = []byte(`{"PatchedResource":{"apiVersion":"v1","kind":"Pod","metadata":{"name":"nginx1","namespace":"default"},"spec":{"containers":[{"image":"nginx","imagePullPolicy":"IfNotPresent","name":"nginx","resources":{"limits":{"cpu":"200m","memory":"100Mi"},"requests":{"cpu":"100m","memory":"50Mi"}}}]}},"PolicyResponse":{"policy":{"name":"pod-requirements","namespace":""},"resource":{"kind":"Pod","apiVersion":"v1","namespace":"default","name":"nginx1","uid":""},"processingTime":974958,"rulesAppliedCount":2,"policyExecutionTimestamp":1630527712,"rules":[{"name":"pods-require-account","type":"Validation","message":"validation error: User pods must include an account for charging. Rule pods-require-account failed at path /metadata/labels/","status":"fail","processingTime":28833,"ruleExecutionTimestamp":1630527712},{"name":"pods-require-limits","type":"Validation","message":"validation rule 'pods-require-limits' passed.","status":"pass","processingTime":578625,"ruleExecutionTimestamp":1630527712}],"ValidationFailureAction":"audit"}}`)

func Test_buildPolicyReports(t *testing.T) {
	rc := &kyvCommon.ResultCounts{}
	var pvInfos []policyreport.Info
	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	var er response.EngineResponse
	err = json.Unmarshal(rawEngRes, &er)
	assert.NilError(t, err)

	info := kyvCommon.ProcessValidateEngineResponse(&policy, &er, "", rc, true)
	pvInfos = append(pvInfos, info)

	reports := buildPolicyReports(pvInfos)
	assert.Assert(t, len(reports) == 1, len(reports))

	for _, report := range reports {
		if report.GetNamespace() == "" {
			assert.Assert(t, report.GetName() == clusterpolicyreport)
			assert.Assert(t, report.GetKind() == "ClusterPolicyReport")
			assert.Assert(t, len(report.UnstructuredContent()["results"].([]interface{})) == 2)
			assert.Assert(t,
				report.UnstructuredContent()["summary"].(map[string]interface{})[preport.StatusPass].(int64) == 1,
				report.UnstructuredContent()["summary"].(map[string]interface{})[preport.StatusPass].(int64))
		} else {
			assert.Assert(t, report.GetName() == "policyreport-ns-default")
			assert.Assert(t, report.GetKind() == "PolicyReport")
			assert.Assert(t, len(report.UnstructuredContent()["results"].([]interface{})) == 2)

			summary := report.UnstructuredContent()["summary"].(map[string]interface{})
			assert.Assert(t, summary[preport.StatusPass].(int64) == 1, summary[preport.StatusPass].(int64))
		}
	}
}

func Test_buildPolicyResults(t *testing.T) {
	rc := &kyvCommon.ResultCounts{}
	var pvInfos []policyreport.Info
	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	var er response.EngineResponse
	err = json.Unmarshal(rawEngRes, &er)
	assert.NilError(t, err)

	info := kyvCommon.ProcessValidateEngineResponse(&policy, &er, "", rc, true)
	pvInfos = append(pvInfos, info)

	results := buildPolicyResults(pvInfos)

	for _, result := range results {
		assert.Assert(t, len(result) == 2, len(result))
		for _, r := range result {
			switch r.Rule {
			case "pods-require-limits":
				assert.Assert(t, r.Result == preport.StatusPass)
			case "pods-require-account":
				assert.Assert(t, r.Result == preport.StatusFail)
			}
		}
	}
}

func Test_calculateSummary(t *testing.T) {
	results := []*preport.PolicyReportResult{
		{
			Resources: make([]*v1.ObjectReference, 5),
			Result:    preport.PolicyResult(preport.StatusPass),
		},
		{Result: preport.PolicyResult(preport.StatusFail)},
		{Result: preport.PolicyResult(preport.StatusFail)},
		{Result: preport.PolicyResult(preport.StatusFail)},
		{
			Resources: make([]*v1.ObjectReference, 1),
			Result:    preport.PolicyResult(preport.StatusPass)},
		{
			Resources: make([]*v1.ObjectReference, 4),
			Result:    preport.PolicyResult(preport.StatusPass),
		},
	}

	summary := calculateSummary(results)
	assert.Assert(t, summary.Pass == 3)
	assert.Assert(t, summary.Fail == 3)
}
