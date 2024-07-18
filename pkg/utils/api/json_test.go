package api

import (
	"reflect"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
)

type Person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestDeserializeJSONArray(t *testing.T) {
	tests := []struct {
		name     string
		input    apiextensions.JSON
		expected []interface{}
		wantErr  bool
	}{
		{
			name:     "empty input",
			input:    nil,
			expected: nil,
			wantErr:  false,
		},
		{
			name:  "valid input",
			input: apiextensions.JSON(`[{"name": "John", "age": 30}, {"name": "Jane", "age": 25}]`),
			expected: []interface{}{
				map[string]interface{}{
					"name": "John",
					"age":  30,
				},
				map[string]interface{}{
					"name": "Jane",
					"age":  25,
				},
			},
			wantErr: false,
		},
		{
			name:     "invalid input",
			input:    apiextensions.JSON(`{"name": "John", "age": 30}`),
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if inputJson, ok := tt.input.(Person); ok {
				got, err := DeserializeJSONArray[Person](inputJson)
				if (err != nil) != tt.wantErr {
					t.Errorf("DeserializeJSONArray() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !reflect.DeepEqual(got, tt.expected) {
					t.Errorf("DeserializeJSONArray() = %v, want %v", got, tt.expected)
				}
			}
		})
	}
}
