package apply

import (
	"encoding/json"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	preport "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"gotest.tools/assert"
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

	clustered, namespaced := buildPolicyReports(false, er)
	assert.Assert(t, len(clustered) == 1, len(clustered))
	assert.Assert(t, len(namespaced) == 0, len(namespaced))
	{
		report := clustered[0]
		assert.Assert(t, report.GetName() == clusterpolicyreport)
		assert.Assert(t, report.Kind == "ClusterPolicyReport")
		assert.Assert(t, len(report.Results) == 2)
		assert.Assert(t, report.Summary.Pass == 1, report.Summary.Pass)
	}
}

func Test_buildPolicyResults(t *testing.T) {
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

	results := buildPolicyResults(false, er)

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
