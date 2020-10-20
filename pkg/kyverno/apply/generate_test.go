package apply

import (
	"reflect"
	"testing"

	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_mergeClusterReport(t *testing.T) {
	reports := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"apiVersion": "policy.kubernetes.io/v1alpha1",
				"kind":       "PolicyReport",
				"metadata": map[string]interface{}{
					"name":      "ns-polr-1",
					"namespace": "ns-polr",
				},
				"results": []interface{}{
					map[string]interface{}{
						"policy":    "ns-polr-1",
						"status":    "pass",
						"resources": make([]interface{}, 10),
					},
				},
			},
		},
		{
			Object: map[string]interface{}{
				"apiVersion": "policy.kubernetes.io/v1alpha1",
				"kind":       "PolicyReport",
				"metadata": map[string]interface{}{
					"name": "ns-polr-2",
				},
				"results": []interface{}{
					map[string]interface{}{
						"policy":    "ns-polr-2",
						"status":    "pass",
						"resources": make([]interface{}, 5),
					},
				},
			},
		},
		{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "polr-3",
				},
				"results": []interface{}{
					map[string]interface{}{
						"policy":    "polr-3",
						"status":    "pass",
						"resources": make([]interface{}, 1),
					},
				},
			},
		},
		{
			Object: map[string]interface{}{
				"apiVersion": "policy.kubernetes.io/v1alpha1",
				"kind":       "ClusterPolicyReport",
				"metadata": map[string]interface{}{
					"name": "cpolr-4",
				},
				"results": []interface{}{
					map[string]interface{}{
						"policy": "cpolr-4",
						"status": "fail",
					},
				},
			},
		},
		{
			Object: map[string]interface{}{
				"apiVersion": "policy.kubernetes.io/v1alpha1",
				"kind":       "ClusterPolicyReport",
				"metadata": map[string]interface{}{
					"name": "cpolr-5",
				},
				"results": []interface{}{
					map[string]interface{}{
						"policy": "cpolr-5",
						"status": "fail",
					},
				},
			},
		},
	}

	expectedResults := []interface{}{
		map[string]interface{}{
			"policy":    "ns-polr-2",
			"status":    "pass",
			"resources": make([]interface{}, 5),
		},
		map[string]interface{}{
			"policy":    "polr-3",
			"status":    "pass",
			"resources": make([]interface{}, 1),
		},
		map[string]interface{}{
			"policy": "cpolr-4",
			"status": "fail",
		},
		map[string]interface{}{
			"policy": "cpolr-5",
			"status": "fail",
		},
	}

	cpolr, err := mergeClusterReport(reports)
	assert.NilError(t, err)

	assert.Assert(t, cpolr.GetAPIVersion() == "policy.kubernetes.io/v1alpha1", cpolr.GetAPIVersion())
	assert.Assert(t, cpolr.GetKind() == "ClusterPolicyReport", cpolr.GetKind())

	entries, _, err := unstructured.NestedSlice(cpolr.UnstructuredContent(), "results")
	assert.NilError(t, err)

	assert.Assert(t, reflect.DeepEqual(entries, expectedResults), entries...)

	summary, _, err := unstructured.NestedMap(cpolr.UnstructuredContent(), "summary")
	assert.NilError(t, err)
	assert.Assert(t, summary["pass"].(int64) == 6, summary["pass"])
	assert.Assert(t, summary["fail"].(int64) == 2, summary["fail"])
}

func Test_updateSummary(t *testing.T) {
	results := []interface{}{
		map[string]interface{}{
			"status":    "pass",
			"resources": make([]interface{}, 5),
		},
		map[string]interface{}{
			"status": "fail",
		},
		map[string]interface{}{
			"status": "fail",
		},
		map[string]interface{}{
			"status": "fail",
		},
	}

	summary := updateSummary(results)
	assert.Assert(t, summary["pass"].(int64) == 5, summary["pass"])
	assert.Assert(t, summary["fail"].(int64) == 3, summary["fail"])
}
