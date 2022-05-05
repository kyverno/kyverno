package policy

import (
	"encoding/json"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"gotest.tools/assert"
)

func Test_valid_onUpdatePolicyPolicy(t *testing.T) {
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "test-gen",
		  "annotations": {
			"policies.kyverno.io/category": "Best Practices"
		  }
		},
		"spec": {
		  "rules": [
			{
			  "match": {
				"resources": {
				  "kinds": [
					"Namespace"
				  ]
				}
			  },
			  "name": "test-gen",
			  "preconditions": {
				"all": [
				  {
					"key": "{{request.object.metadata.name}}",
					"operator": "NotEquals",
					"value": ""
				  }
				]
			  },
			  "validate": {
				"message": "The only label that may be removed or changed is breakglass.",
				"deny": {
				  "conditions": {
					"any": [
					  {
						"key": "{{ request.object.metadata.labels  |  merge(@, {breakglass:null}) }}",
						"operator": "NotEquals",
						"value": "{{ request.oldObject.metadata.labels  |  merge(@, {breakglass:null}) }}"
					  }
					]
				  }
				}
			  }
			}
		  ]
		}
	  }`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	err = ValidateOnPolicyUpdate(&policy, true)
	assert.NilError(t, err)
}

func Test_invalid_onUpdatePolicyPolicy(t *testing.T) {
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "who-created-this"
		},
		"spec": {
		  "rules": [
			{
			  "name": "who-created-this",
			  "match": {
				"any": [
				  {
					"resources": {
					  "kinds": [
						"Pod"
					  ]
					}
				  }
				]
			  },
			  "mutate": {
				"patchStrategicMerge": {
				  "metadata": {
					"labels": {
					  "created-by": "{{request.userInfo.username}}"
					}
				  }
				}
			  }
			}
		  ]
		}
	  }`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)
	err = ValidateOnPolicyUpdate(&policy, true)
	assert.ErrorContains(t, err, "only select variables are allowed in on policy update. Set spec.mutateExistingOnPolicyUpdate=false to disable update policy mode for this policy rule: variable \"{{request.userInfo.username}} is not allowed ")
}
