package common

import (
	"testing"

	"github.com/kyverno/kyverno/api/kyverno/v1beta1"
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
				Skip:  2,
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
				Skip:  4,
			},
		},
	}
	rc := &ResultCounts{}
	for _, tc := range testcases {
		policyArray, _ := yamlutils.GetPolicy(tc.policy)
		resourceArray, _ := GetResource(tc.resource)
		applyPolicyConfig := ApplyPolicyConfig{
			Policy:               policyArray[0],
			Resource:             resourceArray[0],
			MutateLogPath:        "",
			UserInfo:             v1beta1.RequestInfo{},
			NamespaceSelectorMap: tc.namespaceSelectorMap,
			Rc:                   rc,
		}
		ApplyPolicyOnResource(applyPolicyConfig)
		assert.Equal(t, int64(rc.Pass), int64(tc.result.Pass))
		assert.Equal(t, int64(rc.Fail), int64(tc.result.Fail))
		// TODO: autogen rules seem to not be present when autogen internals is disabled
		assert.Equal(t, int64(rc.Skip), int64(tc.result.Skip))
		assert.Equal(t, int64(rc.Warn), int64(tc.result.Warn))
		assert.Equal(t, int64(rc.Error), int64(tc.result.Error))
	}
}

func Test_IsGitSourcePath(t *testing.T) {
	type TestCase struct {
		path    []string
		actual  bool
		desired bool
	}
	testcases := []TestCase{
		{
			path:    []string{"https://github.com/kyverno/policies/openshift/team-validate-ns-name/"},
			desired: true,
		},
		{
			path:    []string{"/kyverno/policies/openshift/team-validate-ns-name/"},
			desired: false,
		},
		{
			path:    []string{"https://bitbucket.org/kyverno/policies/openshift/team-validate-ns-name"},
			desired: true,
		},
		{
			path:    []string{"https://anydomain.com/kyverno/policies/openshift/team-validate-ns-name"},
			desired: true,
		},
	}
	for _, tc := range testcases {
		tc.actual = IsGitSourcePath(tc.path)
		if tc.actual != tc.desired {
			t.Errorf("%s is not a git URL", tc.path)
		}
	}
}

func Test_GetGitBranchOrPolicyPaths(t *testing.T) {
	type TestCase struct {
		gitBranch                             string
		repoURL                               string
		policyPath                            []string
		desiredBranch, actualBranch           string
		desiredPathToYAMLs, actualPathToYAMLs string
	}
	testcases := []TestCase{
		{
			gitBranch:          "main",
			repoURL:            "https://github.com/kyverno/policies",
			policyPath:         []string{"https://github.com/kyverno/policies/openshift/team-validate-ns-name/"},
			desiredBranch:      "main",
			desiredPathToYAMLs: "/openshift/team-validate-ns-name/",
		},
		{
			gitBranch:          "",
			repoURL:            "https://github.com/kyverno/policies",
			policyPath:         []string{"https://github.com/kyverno/policies/"},
			desiredBranch:      "main",
			desiredPathToYAMLs: "/",
		},
		{
			gitBranch:          "",
			repoURL:            "https://github.com/kyverno/policies",
			policyPath:         []string{"https://github.com/kyverno/policies"},
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

	subresources := []Subresource{
		{
			APIResource:    podEvictionAPIResource,
			ParentResource: podAPIResource,
		},
	}

	subresourceKind, err := getSubresourceKind("", "Pod", "eviction", subresources)
	assert.NilError(t, err)
	assert.Equal(t, subresourceKind, "Eviction")
}
