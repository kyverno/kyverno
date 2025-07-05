package resource

import (
	"testing"

	"gotest.tools/assert"
)

func TestNormalizeEmptyFields(t *testing.T) {
	tests := []struct {
		name            string
		input           map[string]interface{}
		dropEmptyFields bool
		expected        map[string]interface{}
		expectError     bool
	}{
		{
			name:            "empty input, drop = true",
			input:           map[string]interface{}{},
			dropEmptyFields: true,
			expected:        map[string]interface{}{},
		},
		{
			name:            "nil field dropped",
			input:           map[string]interface{}{"metadata": nil},
			dropEmptyFields: true,
			expected:        map[string]interface{}{},
		},
		{
			name: "nested nil dropped",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"annotations": nil,
						},
					},
				},
			},
			dropEmptyFields: true,
			expected: map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{},
					},
				},
			},
		},
		{
			name: "list with nil elements dropped",
			input: map[string]interface{}{
				"containers": []interface{}{
					nil,
					map[string]interface{}{
						"env": nil,
					},
				},
			},
			dropEmptyFields: true,
			expected: map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{},
				},
			},
		},
		{
			name:            "error on top-level nil when drop=false",
			input:           map[string]interface{}{"metadata": nil},
			dropEmptyFields: false,
			expectError:     true,
		},
		{
			name: "error on nested nil when drop=false",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"annotations": nil,
						},
					},
				},
			},
			dropEmptyFields: false,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NormalizeEmptyFields(tt.input, tt.dropEmptyFields)

			if tt.expectError {
				assert.Assert(t, err != nil, "expected error but got nil")
			} else {
				assert.NilError(t, err)
				assert.DeepEqual(t, tt.expected, tt.input)
			}
		})
	}
}
