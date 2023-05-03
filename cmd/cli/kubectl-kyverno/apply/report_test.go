package apply

import (
	"encoding/json"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	preport "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	kyvCommon "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
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

func Test_buildPolicyReports(t *testing.T) {
	rc := &kyvCommon.ResultCounts{}
	var pvInfos []common.Info
	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	er := engineapi.EngineResponse{}
	er = er.WithPolicy(&policy)
	er.PolicyResponse.Add(
		engineapi.ExecutionStats{},
		*engineapi.RuleFail(
			"pods-require-account",
			engineapi.Validation,
			"validation error: User pods must include an account for charging. Rule pods-require-account failed at path /metadata/labels/",
		),
		*engineapi.RulePass(
			"pods-require-limits",
			engineapi.Validation,
			"validation rule 'pods-require-limits' passed.",
		),
	)

	info := kyvCommon.ProcessValidateEngineResponse(&policy, &er, "", rc, true, false)
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
	var pvInfos []common.Info
	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	er := engineapi.EngineResponse{}
	er = er.WithPolicy(&policy)
	er.PolicyResponse.Add(
		engineapi.ExecutionStats{}, *engineapi.RuleFail(
			"pods-require-account",
			engineapi.Validation,
			"validation error: User pods must include an account for charging. Rule pods-require-account failed at path /metadata/labels/",
		),
		*engineapi.RulePass(
			"pods-require-limits",
			engineapi.Validation,
			"validation rule 'pods-require-limits' passed.",
		),
	)

	info := kyvCommon.ProcessValidateEngineResponse(&policy, &er, "", rc, true, false)
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
	results := []preport.PolicyReportResult{
		{
			Resources: make([]v1.ObjectReference, 5),
			Result:    preport.PolicyResult(preport.StatusPass),
		},
		{Result: preport.PolicyResult(preport.StatusFail)},
		{Result: preport.PolicyResult(preport.StatusFail)},
		{Result: preport.PolicyResult(preport.StatusFail)},
		{
			Resources: make([]v1.ObjectReference, 1),
			Result:    preport.PolicyResult(preport.StatusPass)},
		{
			Resources: make([]v1.ObjectReference, 4),
			Result:    preport.PolicyResult(preport.StatusPass),
		},
	}

	summary := calculateSummary(results)
	assert.Assert(t, summary.Pass == 3)
	assert.Assert(t, summary.Fail == 3)
}
