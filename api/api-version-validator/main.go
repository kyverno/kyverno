package main

import (
	"fmt"
	"os"
	"reflect"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
)

func main() {
	exitCode := 0
	defer func() {
		os.Exit(exitCode)
	}()

	// Compare Policy types
	v1Fields := getStructFields(reflect.TypeOf(kyvernov1.Policy{}))
	v2beta1Fields := getStructFields(reflect.TypeOf(kyvernov2beta1.Policy{}))

	// Compare ClusterPolicy types
	v1ClusterFields := getStructFields(reflect.TypeOf(kyvernov1.ClusterPolicy{}))
	v2beta1ClusterFields := getStructFields(reflect.TypeOf(kyvernov2beta1.ClusterPolicy{}))

	mismatches := validateFields(v1Fields, v2beta1Fields, "Policy")
	mismatches = append(mismatches, validateFields(v1ClusterFields, v2beta1ClusterFields, "ClusterPolicy")...)

	if len(mismatches) > 0 {
		fmt.Println("API version field mismatches found:")
		for _, mismatch := range mismatches {
			fmt.Printf("- %s\n", mismatch)
		}
		exitCode = 1
	}
}

func getStructFields(t reflect.Type) map[string]reflect.StructField {
	fields := make(map[string]reflect.StructField)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !isDeprecated(field) {
			fields[field.Name] = field
		}
	}
	return fields
}

func isDeprecated(field reflect.StructField) bool {
	// Check for deprecated tag or comment
	return field.Tag.Get("deprecated") != ""
}

func validateFields(v1Fields, v2beta1Fields map[string]reflect.StructField, typeName string) []string {
	var mismatches []string

	for name, v1Field := range v1Fields {
		v2Field, exists := v2beta1Fields[name]
		if !exists {
			mismatches = append(mismatches, fmt.Sprintf("%s: field %q exists in v1 but missing in v2beta1", typeName, name))
			continue
		}

		if v1Field.Type != v2Field.Type {
			mismatches = append(mismatches, fmt.Sprintf("%s: field %q has different types - v1: %v, v2beta1: %v",
				typeName, name, v1Field.Type, v2Field.Type))
		}

		if v1Field.Tag != v2Field.Tag {
			mismatches = append(mismatches, fmt.Sprintf("%s: field %q has different tags - v1: %q, v2beta1: %q",
				typeName, name, v1Field.Tag, v2Field.Tag))
		}
	}

	return mismatches
}
