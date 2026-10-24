package v1

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_ValidateGenerate_NamespacedPolicy(t *testing.T) {
	testCases := []struct {
		name          string
		rule          string
		expectWarning bool
		expectError   bool
	}{
		{
			name:          "clone variables produce warning",
			rule:          `{"generate":{"apiVersion":"v1","kind":"Secret","name":"secret","namespace":"default","clone":{"namespace":"default","name":"{{request.object.metadata.name}}"}}}`,
			expectWarning: true,
		},
		{
			name:        "clone source must match policy namespace",
			rule:        `{"generate":{"apiVersion":"v1","kind":"Secret","name":"secret","namespace":"default","clone":{"namespace":"other","name":"secret"}}}`,
			expectError: true,
		},
		{
			name:          "cloneList variables produce warning",
			rule:          `{"generate":{"namespace":"default","synchronize":true,"cloneList":{"namespace":"default","kinds":["v1/Secret","v1/ConfigMap"],"selector":{"matchLabels":{"{{request.object.metadata.name}}":"copy"}}}}}`,
			expectWarning: true,
		},
		{
			name:        "cloneList source must match policy namespace",
			rule:        `{"generate":{"namespace":"default","cloneList":{"namespace":"other","kinds":["v1/Secret"]}}}`,
			expectError: true,
		},
		{
			name:        "cloneList source must be namespaced",
			rule:        `{"generate":{"namespace":"default","cloneList":{"namespace":"default","kinds":["v1/Namespace"]}}}`,
			expectError: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var rule Rule
			assert.NilError(t, json.Unmarshal([]byte(testCase.rule), &rule))
			warnings, errs := rule.ValidateGenerate(field.NewPath("dummy"), true, "default", sets.New("v1/Namespace"))
			assert.Equal(t, len(warnings) > 0, testCase.expectWarning)
			assert.Equal(t, len(errs) > 0, testCase.expectError, errs)
		})
	}
}
