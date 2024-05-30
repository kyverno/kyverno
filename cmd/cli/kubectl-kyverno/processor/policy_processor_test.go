package processor

import (
	"os"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/pkg/registryclient"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
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
	type resultCounts struct {
		pass int
		fail int
		warn int
		err  int
		skip int
	}
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
				pass: 0,
				fail: 1,
				warn: 0,
				err:  0,
				skip: 0,
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
				pass: 1,
				fail: 1,
				warn: 0,
				err:  0,
				skip: 0,
			},
		},
	}
	rc := &ResultCounts{}
	for _, tc := range testcases {
		policyArray, _, _ := yamlutils.GetPolicy(tc.policy)
		resourceArray, _ := resource.GetUnstructuredResources(tc.resource)
		processor := PolicyProcessor{
			Policies:             policyArray,
			Resource:             *resourceArray[0],
			MutateLogPath:        "",
			UserInfo:             nil,
			NamespaceSelectorMap: tc.namespaceSelectorMap,
			Rc:                   rc,
			Out:                  os.Stdout,
			RegistryClient:       registryclient.NewOrDie(),
		}
		processor.ApplyPoliciesOnResource()
		assert.Equal(t, int64(rc.Pass()), int64(tc.result.pass))
		assert.Equal(t, int64(rc.Fail()), int64(tc.result.fail))
		assert.Equal(t, int64(rc.Skip()), int64(tc.result.skip))
		assert.Equal(t, int64(rc.Warn()), int64(tc.result.warn))
		assert.Equal(t, int64(rc.Error()), int64(tc.result.err))
	}
}
