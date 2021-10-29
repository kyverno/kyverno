package policy

import (
	"encoding/json"
	"strings"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"gotest.tools/assert"
)

func Test_Validation_valid_backgroundPolicy(t *testing.T) {
	rawPolicy := []byte(`
		{
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
				  "context": [
					{
					  "name": "mycm",
					  "configMap": {
						"name": "config-name",
						"namespace": "default"
					  }
					}
				  ],
				  "generate": {
					"kind": "ConfigMap",
					"name": "{{request.object.metadata.name}}-config-name",
					"namespace": "{{request.object.metadata.name}}",
					"data": {
					  "data": {
						"new": "{{ mycm.data.foo }}"
					  }
					}
				  }
				}
			  ]
			}
		  }
		`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	err = ContainsVariablesOtherThanObject(policy)
	assert.NilError(t, err)
}

func Test_Validation_invalid_backgroundPolicy(t *testing.T) {
	rawPolicy := []byte(`
		{
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
				  "context": [
					{
					  "name": "mycm",
					  "configMap": {
						"name": "config-name",
						"namespace": "default"
					  }
					}
				  ],
				  "generate": {
					"kind": "ConfigMap",
					"name": "{{serviceAccountName}}-config-name",
					"namespace": "{{serviceAccountName}}",
					"data": {
					  "data": {
						"new": "{{ mycm.data.foo }}"
					  }
					}
				  }
				}
			  ]
			}
		  }
		`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)
	err = ContainsVariablesOtherThanObject(policy)
	assert.Assert(t, strings.Contains(err.Error(), "variable serviceAccountName cannot be used, allowed variables: [request.object request.namespace images element mycm]"))
}
