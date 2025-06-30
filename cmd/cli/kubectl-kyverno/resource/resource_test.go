package resource

import (
	"testing"

	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNormalizeEmptyFields(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name:     "empty input",
			input:    map[string]interface{}{},
			expected: map[string]interface{}{},
		},
		{
			name: "nil field",
			input: map[string]interface{}{
				"metadata": nil,
			},
			expected: map[string]interface{}{
				"metadata": map[string]interface{}{},
			},
		},
		{
			name: "nested nil in deployment",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"annotations": nil,
						},
					},
				},
			},
			expected: map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"annotations": map[string]interface{}{},
						},
					},
				},
			},
		},
		{
			name: "list with nil elements",
			input: map[string]interface{}{
				"containers": []interface{}{
					nil,
					map[string]interface{}{
						"env": nil,
					},
				},
			},
			expected: map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{},
					map[string]interface{}{
						"env": map[string]interface{}{},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := &unstructured.Unstructured{Object: tt.input}
			normalizeEmptyFields(resource.Object)

			assert.DeepEqual(t, tt.expected, resource.Object)
		})
	}
}
