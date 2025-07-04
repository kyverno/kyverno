package utils

import (
	"strings"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_InvertElements(t *testing.T) {
	elems := []interface{}{"a", "b", "c"}
	elemsInverted := InvertElements(elems)

	assert.Equal(t, "a", elemsInverted[2])
	assert.Equal(t, "b", elemsInverted[1])
	assert.Equal(t, "c", elemsInverted[0])
}

func Test_EvaluateList(t *testing.T) {
	entryName := "test_object"
	cases := []struct {
		name     string
		rawData  []byte
		jmesPath string
		expected interface{}
	}{
		{
			name:     "slice data",
			rawData:  []byte(`["test-value-1", "test-value-2"]`),
			jmesPath: entryName,
			expected: []interface{}{"test-value-1", "test-value-2"},
		},
		{
			name: "map data",
			rawData: []byte(`
				{
					"test-key-1": "test-value-1",
					"test-key-2": "test-value-2"
				}
			`),
			jmesPath: entryName + ".items(@, 'key', 'value')",
			expected: []interface{}{
				map[string]interface{}{
					"key":   "test-key-1",
					"value": "test-value-1",
				},
				map[string]interface{}{
					"key":   "test-key-2",
					"value": "test-value-2",
				},
			},
		},
		{
			name: "map data with custom fields",
			rawData: []byte(`
				{
					"test-key-1": "test-value-1",
					"test-key-2": "test-value-2"
				}
			`),
			jmesPath: "test_object.items(@, 'another-key', 'another-value')",
			expected: []interface{}{
				map[string]interface{}{
					"another-key":   "test-key-1",
					"another-value": "test-value-1",
				},
				map[string]interface{}{
					"another-key":   "test-key-2",
					"another-value": "test-value-2",
				},
			},
		},
	}

	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)

	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			ctx := context.NewContext(jp)
			assert.NoError(t, ctx.AddContextEntry(entryName, item.rawData))

			list, err := EvaluateList(item.jmesPath, ctx)
			assert.NoError(t, err)
			assert.Equal(t, item.expected, list)
		})
	}
}

var orderAscending = kyvernov1.Ascending
var orderDescending = kyvernov1.Descending

func Test_ReversePatchedListIfAscending(t *testing.T) {
	tests := []struct {
		name          string
		inputResource *unstructured.Unstructured
		foreach       kyvernov1.ForEachMutation
		expectedOrder []string
	}{
		{
			inputResource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"initContainers": []interface{}{
							map[string]interface{}{"name": "a"},
							map[string]interface{}{"name": "b"},
							map[string]interface{}{"name": "c"},
						},
					},
				},
			},
			foreach: kyvernov1.ForEachMutation{
				List:  "request.object.spec.initContainers[]",
				Order: &orderAscending,
			},
			expectedOrder: []string{"c", "b", "a"},
		},
		{
			inputResource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{"name": "a"},
						},
					},
				},
			},
			foreach: kyvernov1.ForEachMutation{
				List:  "request.object.spec.containers",
				Order: &orderDescending,
			},
			expectedOrder: []string{"a"},
		},
		{
			inputResource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{"name": "a"},
						},
					},
				},
			},
			foreach: kyvernov1.ForEachMutation{
				List: "request.object.spec.containers",
			},
			expectedOrder: []string{"a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := tt.inputResource.DeepCopy()
			ReversePatchedListIfAscending(tt.foreach, resource)
			field := strings.TrimSuffix(strings.TrimPrefix(tt.foreach.List, "request.object.spec."), "[]")
			containers, _, _ := unstructured.NestedSlice(resource.Object, "spec", field)
			for i, expected := range tt.expectedOrder {
				name := containers[i].(map[string]interface{})["name"].(string)
				assert.Equal(t, expected, name)
			}
		})
	}
}
