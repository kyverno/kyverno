package jsonutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testStruct struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func Test_DocumentToUntyped(t *testing.T) {
	testCases := []struct {
		name      string
		input     interface{}
		expectErr bool
		verify    func(t *testing.T, output interface{})
	}{
		{
			name:      "string-input",
			input:     "test-string",
			expectErr: false,
			verify: func(t *testing.T, output interface{}) {
				assert.Equal(t, "test-string", output)
			},
		},
		{
			name:      "map-input",
			input:     map[string]interface{}{"key": "value"},
			expectErr: false,
			verify: func(t *testing.T, output interface{}) {
				assert.Equal(t, map[string]interface{}{"key": "value"}, output)
			},
		},
		{
			name:      "slice-input",
			input:     []interface{}{"a", "b"},
			expectErr: false,
			verify: func(t *testing.T, output interface{}) {
				assert.Equal(t, []interface{}{"a", "b"}, output)
			},
		},
		{
			name:      "struct-conversion",
			input:     testStruct{Name: "Kyverno", Value: 10},
			expectErr: false,
			verify: func(t *testing.T, output interface{}) {
				expected := map[string]interface{}{
					"name":  "Kyverno",
					"value": float64(10), // json.Unmarshal converts numbers to float64 by default
				}
				assert.Equal(t, expected, output)
			},
		},
		{
			name:      "nil-input",
			input:     nil,
			expectErr: false,
			verify: func(t *testing.T, output interface{}) {
				assert.Nil(t, output)
			},
		},
		{
			name:      "pointer-to-struct",
			input:     &testStruct{Name: "Pointer", Value: 1},
			expectErr: false,
			verify: func(t *testing.T, output interface{}) {
				expected := map[string]interface{}{
					"name":  "Pointer",
					"value": float64(1),
				}
				assert.Equal(t, expected, output)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := DocumentToUntyped(tc.input)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tc.verify(t, res)
			}
		})
	}
}
