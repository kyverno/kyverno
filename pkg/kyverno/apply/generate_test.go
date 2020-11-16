package apply

import (
	"reflect"
	"testing"

	report "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_mergeClusterReport(t *testing.T) {
	reports := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"apiVersion": report.SchemeGroupVersion.String(),
				"kind":       "PolicyReport",
				"metadata": map[string]interface{}{
					"name":      "ns-polr-1",
					"namespace": "ns-polr",
				},
				"results": []interface{}{
					map[string]interface{}{
						"policy":    "ns-polr-1",
						"status":    report.StatusPass,
						"resources": make([]interface{}, 10),
					},
				},
			},
		},
		{
			Object: map[string]interface{}{
				"apiVersion": report.SchemeGroupVersion.String(),
				"kind":       "PolicyReport",
				"metadata": map[string]interface{}{
					"name": "ns-polr-2",
				},
				"results": []interface{}{
					map[string]interface{}{
						"policy":    "ns-polr-2",
						"status":    report.StatusPass,
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
						"status":    report.StatusPass,
						"resources": make([]interface{}, 1),
					},
				},
			},
		},
		{
			Object: map[string]interface{}{
				"apiVersion": report.SchemeGroupVersion.String(),
				"kind":       "ClusterPolicyReport",
				"metadata": map[string]interface{}{
					"name": "cpolr-4",
				},
				"results": []interface{}{
					map[string]interface{}{
						"policy": "cpolr-4",
						"status": report.StatusFail,
					},
				},
			},
		},
		{
			Object: map[string]interface{}{
				"apiVersion": report.SchemeGroupVersion.String(),
				"kind":       "ClusterPolicyReport",
				"metadata": map[string]interface{}{
					"name": "cpolr-5",
				},
				"results": []interface{}{
					map[string]interface{}{
						"policy": "cpolr-5",
						"status": report.StatusFail,
					},
				},
			},
		},
	}

	expectedResults := []interface{}{
		map[string]interface{}{
			"policy":    "ns-polr-2",
			"status":    report.StatusPass,
			"resources": make([]interface{}, 5),
		},
		map[string]interface{}{
			"policy":    "polr-3",
			"status":    report.StatusPass,
			"resources": make([]interface{}, 1),
		},
		map[string]interface{}{
			"policy": "cpolr-4",
			"status": report.StatusFail,
		},
		map[string]interface{}{
			"policy": "cpolr-5",
			"status": report.StatusFail,
		},
	}

	cpolr, err := mergeClusterReport(reports)
	assert.NilError(t, err)

	assert.Assert(t, cpolr.GetAPIVersion() == report.SchemeGroupVersion.String(), cpolr.GetAPIVersion())
	assert.Assert(t, cpolr.GetKind() == "ClusterPolicyReport", cpolr.GetKind())

	entries, _, err := unstructured.NestedSlice(cpolr.UnstructuredContent(), "results")
	assert.NilError(t, err)

	assert.Assert(t, reflect.DeepEqual(entries, expectedResults), entries...)

	summary, _, err := unstructured.NestedMap(cpolr.UnstructuredContent(), "summary")
	assert.NilError(t, err)
	assert.Assert(t, summary[report.StatusPass].(int64) == 6, summary[report.StatusPass])
	assert.Assert(t, summary[report.StatusFail].(int64) == 2, summary[report.StatusFail])
}

func Test_updateSummary(t *testing.T) {
	results := []interface{}{
		map[string]interface{}{
			"status":    report.StatusPass,
			"resources": make([]interface{}, 5),
		},
		map[string]interface{}{
			"status": report.StatusFail,
		},
		map[string]interface{}{
			"status": report.StatusFail,
		},
		map[string]interface{}{
			"status": report.StatusFail,
		},
	}

	summary := updateSummary(results)
	assert.Assert(t, summary[report.StatusPass].(int64) == 5, summary[report.StatusPass])
	assert.Assert(t, summary[report.StatusFail].(int64) == 3, summary[report.StatusFail])
}
