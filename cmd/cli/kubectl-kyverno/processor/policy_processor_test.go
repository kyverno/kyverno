package processor

import (
	"os"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/restmapper"
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
		policyArray, _, _, _, _, _, _, _ := yamlutils.GetPolicy(tc.policy)
		resourceArray, _ := resource.GetUnstructuredResources(tc.resource)
		processor := PolicyProcessor{
			Store:                &store.Store{},
			Policies:             policyArray,
			Resource:             *resourceArray[0],
			MutateLogPath:        "",
			UserInfo:             nil,
			NamespaceSelectorMap: tc.namespaceSelectorMap,
			Rc:                   rc,
			Out:                  os.Stdout,
		}
		processor.ApplyPoliciesOnResource()
		assert.Equal(t, int64(rc.Pass), int64(tc.result.Pass))
		assert.Equal(t, int64(rc.Fail), int64(tc.result.Fail))
		assert.Equal(t, int64(rc.Skip), int64(tc.result.Skip))
		assert.Equal(t, int64(rc.Warn), int64(tc.result.Warn))
		assert.Equal(t, int64(rc.Error), int64(tc.result.Error))
	}
}

var validatingPolicyWithCRD = []byte(`
{
  "apiVersion": "policies.kyverno.io/v1",
  "kind": "ValidatingPolicy",
  "metadata": {
    "name": "require-widget-label"
  },
  "spec": {
    "validationActions": [
      "Audit"
    ],
    "matchConstraints": {
      "resourceRules": [
        {
          "apiGroups": [
            "example.com"
          ],
          "apiVersions": [
            "v1"
          ],
          "operations": [
            "CREATE",
            "UPDATE"
          ],
          "resources": [
            "widgets"
          ]
        }
      ]
    },
    "validations": [
      {
        "expression": "has(object.metadata.labels) && has(object.metadata.labels.app)",
        "message": "Widget must have an app label."
      }
    ]
  }
}
`)

var widgetResourceJSON = []byte(`{
    "apiVersion": "example.com/v1",
    "kind": "Widget",
    "metadata": {
      "name": "good-widget",
      "namespace": "default",
      "labels": {
        "app": "my-app"
      }
    }
  }
`)

func Test_RESTMapper(t *testing.T) {
	_, _, _, vpols, _, _, _, err := yamlutils.GetPolicy(validatingPolicyWithCRD)
	assert.NilError(t, err)
	assert.Equal(t, 1, len(vpols))
	vpolsLike := make([]policiesv1beta1.ValidatingPolicyLike, len(vpols))
	for i := range vpols {
		vpolsLike[i] = &vpols[i]
	}

	widgets, err := resource.GetUnstructuredResources(widgetResourceJSON)
	assert.NilError(t, err)
	assert.Equal(t, 1, len(widgets))

	defaultNS := &unstructured.Unstructured{}
	defaultNS.SetName("default")
	namespaceCache := map[string]*unstructured.Unstructured{"default": defaultNS}

	t.Run("with_RESTMapper", func(t *testing.T) {
		rc := &ResultCounts{}
		rm := restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{
			{
				Group: metav1.APIGroup{
					Name:     "example.com",
					Versions: []metav1.GroupVersionForDiscovery{{GroupVersion: "example.com/v1", Version: "v1"}},
				},
				VersionedResources: map[string][]metav1.APIResource{
					"v1": {{Name: "widgets", Namespaced: true, Kind: "Widget"}},
				},
			},
		})

		proc := PolicyProcessor{
			Store:              &store.Store{},
			ValidatingPolicies: vpolsLike,
			Resource:           *widgets[0],
			Rc:                 rc,
			Out:                os.Stdout,
			Cluster:            true,
			NamespaceCache:     namespaceCache,
			Variables:          &variables.Variables{},
			RESTMapper:         rm,
		}
		_, err := proc.ApplyPoliciesOnResource()
		assert.NilError(t, err)
		assert.Equal(t, int64(1), int64(rc.Pass))
	})

	t.Run("nil_RESTMapper_cluster_mode", func(t *testing.T) {
		rc := &ResultCounts{}
		proc := PolicyProcessor{
			Store:              &store.Store{},
			ValidatingPolicies: vpolsLike,
			Resource:           *widgets[0],
			Rc:                 rc,
			Out:                os.Stdout,
			Cluster:            true,
			NamespaceCache:     namespaceCache,
			Variables:          &variables.Variables{},
			RESTMapper:         nil, // Omit the RESTMapper so the built-in default is used
		}
		_, err := proc.ApplyPoliciesOnResource()
		assert.Assert(t, err != nil)
	})
}
