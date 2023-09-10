package common

import (
	"testing"

	valuesapi "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/values"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/processor"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	type ResultCounts struct {
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
				skip: 2,
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
				skip: 4,
			},
		},
	}
	rc := &processor.ResultCounts{}
	for _, tc := range testcases {
		policyArray, _, _ := yamlutils.GetPolicy(tc.policy)
		resourceArray, _ := resource.GetUnstructuredResources(tc.resource)
		processor := processor.PolicyProcessor{
			Policies:             policyArray,
			Resource:             resourceArray[0],
			MutateLogPath:        "",
			UserInfo:             nil,
			NamespaceSelectorMap: tc.namespaceSelectorMap,
			Rc:                   rc,
		}
		processor.ApplyPoliciesOnResource()
		assert.Equal(t, int64(rc.Pass()), int64(tc.result.pass))
		assert.Equal(t, int64(rc.Fail()), int64(tc.result.fail))
		assert.Equal(t, int64(rc.Skip()), int64(tc.result.skip))
		assert.Equal(t, int64(rc.Warn()), int64(tc.result.warn))
		assert.Equal(t, int64(rc.Error()), int64(tc.result.err))
	}
}

func Test_GetGitBranchOrPolicyPaths(t *testing.T) {
	type TestCase struct {
		gitBranch                             string
		repoURL                               string
		policyPath                            string
		desiredBranch, actualBranch           string
		desiredPathToYAMLs, actualPathToYAMLs string
	}
	testcases := []TestCase{
		{
			gitBranch:          "main",
			repoURL:            "https://github.com/kyverno/policies",
			policyPath:         "https://github.com/kyverno/policies/openshift/team-validate-ns-name/",
			desiredBranch:      "main",
			desiredPathToYAMLs: "/openshift/team-validate-ns-name/",
		},
		{
			gitBranch:          "",
			repoURL:            "https://github.com/kyverno/policies",
			policyPath:         "https://github.com/kyverno/policies/",
			desiredBranch:      "main",
			desiredPathToYAMLs: "/",
		},
		{
			gitBranch:          "",
			repoURL:            "https://github.com/kyverno/policies",
			policyPath:         "https://github.com/kyverno/policies",
			desiredBranch:      "main",
			desiredPathToYAMLs: "/",
		},
	}

	for _, tc := range testcases {
		tc.actualBranch, tc.actualPathToYAMLs = GetGitBranchOrPolicyPaths(tc.gitBranch, tc.repoURL, tc.policyPath)
		if tc.actualBranch != tc.desiredBranch || tc.actualPathToYAMLs != tc.desiredPathToYAMLs {
			t.Errorf("Want %q got %q  OR Want %q got %q", tc.desiredBranch, tc.actualBranch, tc.desiredPathToYAMLs, tc.actualPathToYAMLs)
		}
	}
}

func Test_getSubresourceKind(t *testing.T) {
	podAPIResource := metav1.APIResource{Name: "pods", SingularName: "", Namespaced: true, Kind: "Pod"}
	podEvictionAPIResource := metav1.APIResource{Name: "pods/eviction", SingularName: "", Namespaced: true, Group: "policy", Version: "v1", Kind: "Eviction"}

	subresources := []valuesapi.Subresource{
		{
			APIResource:    podEvictionAPIResource,
			ParentResource: podAPIResource,
		},
	}

	subresourceKind, err := getSubresourceKind("", "Pod", "eviction", subresources)
	assert.NilError(t, err)
	assert.Equal(t, subresourceKind, "Eviction")
}
