package main

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetStructFields(t *testing.T) {
	type TestStruct struct {
		Normal     string
		Deprecated string `deprecated:"true"`
	}

	fields := getStructFields(reflect.TypeOf(TestStruct{}))
	require.Contains(t, fields, "Normal")
	require.NotContains(t, fields, "Deprecated")
}

func TestValidateFields(t *testing.T) {
	tests := []struct {
		name           string
		v1Fields       map[string]reflect.StructField
		v2beta1Fields  map[string]reflect.StructField
		typeName       string
		expectMismatch bool
	}{
		{
			name: "matching fields",
			v1Fields: map[string]reflect.StructField{
				"Test": {Name: "Test", Type: reflect.TypeOf("")},
			},
			v2beta1Fields: map[string]reflect.StructField{
				"Test": {Name: "Test", Type: reflect.TypeOf("")},
			},
			typeName:       "Policy",
			expectMismatch: false,
		},
		{
			name: "missing field in v2beta1",
			v1Fields: map[string]reflect.StructField{
				"Test": {Name: "Test", Type: reflect.TypeOf("")},
			},
			v2beta1Fields:  map[string]reflect.StructField{},
			typeName:       "Policy",
			expectMismatch: true,
		},
		{
			name: "type mismatch",
			v1Fields: map[string]reflect.StructField{
				"Test": {Name: "Test", Type: reflect.TypeOf("")},
			},
			v2beta1Fields: map[string]reflect.StructField{
				"Test": {Name: "Test", Type: reflect.TypeOf(1)},
			},
			typeName:       "Policy",
			expectMismatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mismatches := validateFields(tt.v1Fields, tt.v2beta1Fields, tt.typeName)
			if tt.expectMismatch {
				require.NotEmpty(t, mismatches)
			} else {
				require.Empty(t, mismatches)
			}
		})
	}
}
