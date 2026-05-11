package processor

import (
	"os"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	cliapiv1alpha1 "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// Test_makePolicyContext_OldObjectReplaceSemantics verifies that when values supply
// request.oldObject as a full object map, it replaces (not merges with) the default
// old resource derived from the new resource, so no fields from the new object leak
// into request.oldObject.
func Test_makePolicyContext_OldObjectReplaceSemantics(t *testing.T) {
	newObj := unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "nginx",
			"namespace": "test1",
			// new object has env=prod and an extra label absent from oldObject
			"labels": map[string]interface{}{"env": "prod", "extra": "new-only"},
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{"name": "c", "image": "nginx:latest"},
			},
		},
	}}

	// oldObject intentionally omits "extra" to prove replacement, not merge
	oldObjectMap := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "nginx",
			"namespace": "test1",
			"labels":    map[string]interface{}{"env": "dev"},
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{"name": "c", "image": "nginx:latest"},
			},
		},
	}

	valuesSpec := &cliapiv1alpha1.ValuesSpec{
		Policies: []cliapiv1alpha1.Policy{{
			Name: "enforce-pod-name",
			Resources: []cliapiv1alpha1.Resource{{
				Name: "nginx",
				Values: map[string]interface{}{
					"request.operation": "UPDATE",
					"request.oldObject": oldObjectMap,
				},
			}},
		}},
	}

	vars, err := variables.New(os.Stdout, nil, "", "", valuesSpec)
	assert.NilError(t, err)

	policyArray, _, _, _, _, _, _, _ := yamlutils.GetPolicy(policyNamespaceSelector)
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	cfg := config.NewDefaultConfiguration(false)

	p := &PolicyProcessor{
		Store:     &store.Store{},
		Variables: vars,
	}
	ctx, err := p.makePolicyContext(jp, cfg, newObj, policyArray[0], nil,
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}, "")
	assert.NilError(t, err)

	assert.Equal(t, kyvernov1.Update, ctx.Operation())

	// new resource retains its labels
	newRes := ctx.NewResource()
	assert.Equal(t, "prod", newRes.GetLabels()["env"])
	assert.Equal(t, "new-only", newRes.GetLabels()["extra"])

	// old resource must reflect oldObjectMap exactly — env=dev, no "extra"
	oldRes := ctx.OldResource()
	assert.Equal(t, "dev", oldRes.GetLabels()["env"])
	_, hasExtra := oldRes.GetLabels()["extra"]
	assert.Equal(t, false, hasExtra, "old resource must not retain 'extra' label from new object")

	// JSON context must also reflect replacement
	retOld, err := ctx.JSONContext().Query("request.oldObject.metadata.labels.env")
	assert.NilError(t, err)
	assert.Equal(t, "dev", retOld)

	retNew, err := ctx.JSONContext().Query("request.object.metadata.labels.env")
	assert.NilError(t, err)
	assert.Equal(t, "prod", retNew)
}

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
