package common

import (
	"testing"

	ut "github.com/kyverno/kyverno/pkg/utils"
	"gotest.tools/assert"
)

var policyNamespaceSelector = []byte(`{
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
		result               ResultCounts
	}

	testcases := []TestCase{
		{
			policy:   policyNamespaceSelector,
			resource: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"nginx","namespace":"test1"},"spec":{"containers":[{"image":"nginx:latest","name":"test-fail"}]}}`),
			namespaceSelectorMap: map[string]map[string]string{
				"test1": {
					"foo.com/managed-state": "managed",
				},
			},
			result: ResultCounts{
				Pass:  0,
				Fail:  1,
				Warn:  0,
				Error: 0,
				Skip:  0,
			},
		},
		{
			policy:   policyNamespaceSelector,
			resource: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test-nginx","namespace":"test1"},"spec":{"containers":[{"image":"nginx:latest","name":"test-pass"}]}}`),
			namespaceSelectorMap: map[string]map[string]string{
				"test1": {
					"foo.com/managed-state": "managed",
				},
			},
			result: ResultCounts{
				Pass:  1,
				Fail:  1,
				Warn:  0,
				Error: 0,
				Skip:  0,
			},
		},
	}

	rc := &ResultCounts{}
	for _, tc := range testcases {
		policyArray, _ := ut.GetPolicy(tc.policy)
		resourceArray, _ := GetResource(tc.resource)
		ApplyPolicyOnResource(policyArray[0], resourceArray[0], "", false, nil, false, tc.namespaceSelectorMap, false, rc, false)
		assert.Assert(t, int64(rc.Pass) == int64(tc.result.Pass))
		assert.Assert(t, int64(rc.Fail) == int64(tc.result.Fail))
		assert.Assert(t, int64(rc.Skip) == int64(tc.result.Skip))
		assert.Assert(t, int64(rc.Warn) == int64(tc.result.Warn))
		assert.Assert(t, int64(rc.Error) == int64(tc.result.Error))
	}
}
