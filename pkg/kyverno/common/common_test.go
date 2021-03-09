package common

import (
	"testing"

	ut "github.com/kyverno/kyverno/pkg/utils"
	"gotest.tools/assert"
)

var policyNamespaceSeelector = []byte(`{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
	  "name": "enforce-pod-name"
	},
	"spec": {
	  "validationFailureAction": "audit",
	  "background": true,
	  "rules": [
		{
		  "name": "validate-name",
		  "match": {
			"resources": {
			  "kinds": [
				"Pod"
			  ],
			  "namespaceSelector": {
				"matchExpressions": [
				  {
					"key": "foo.com/managed-state",
					"operator": "In",
					"values": [
					  "managed"
					]
				  }
				]
			  }
			}
		  },
		  "validate": {
			"message": "The Pod must end with -nginx",
			"pattern": {
			  "metadata": {
				"name": "*-nginx"
			  }
			}
		  }
		}
	  ]
	}
  }
`)

func Test_NamespaceSelector(t *testing.T) {
	type TestCase struct {
		policy               []byte
		resource             []byte
		namespaceSelectorMap map[string]map[string]string
		sucess               bool
	}

	testcases := []TestCase{
		{
			policy:   policyNamespaceSeelector,
			resource: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"nginx","namespace":"test1"},"spec":{"containers":[{"image":"nginx:latest","name":"test-fail"}]}}`),
			namespaceSelectorMap: map[string]map[string]string{
				"test1": map[string]string{
					"foo.com/managed-state": "managed",
				},
			},
			sucess: false,
		},
		{
			policy:   policyNamespaceSeelector,
			resource: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test-nginx","namespace":"test1"},"spec":{"containers":[{"image":"nginx:latest","name":"test-pass"}]}}`),
			namespaceSelectorMap: map[string]map[string]string{
				"test1": map[string]string{
					"foo.com/managed-state": "managed",
				},
			},
			sucess: true,
		},
	}

	for _, tc := range testcases {
		policyArray, _ := ut.GetPolicy(tc.policy)
		resourceArray, _ := GetResource(tc.resource)
		_, validateErs, _, _, _ := ApplyPolicyOnResource(policyArray[0], resourceArray[0], "", false, nil, false, tc.namespaceSelectorMap)
		assert.Assert(t, tc.sucess == validateErs.IsSuccessful())
	}
}
