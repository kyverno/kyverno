package utils

import (
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestMatchesValue(t *testing.T) {
	testCases := []struct {
		name           string
		value          string
		exemptedValues []string
		operator       kyvernov2.ValueOperator
		expected       bool
	}{
		{
			name:           "equals operator - match",
			value:          "debug",
			exemptedValues: []string{"debug", "test"},
			operator:       kyvernov2.ValueOperatorEquals,
			expected:       true,
		},
		{
			name:           "equals operator - no match",
			value:          "production",
			exemptedValues: []string{"debug", "test"},
			operator:       kyvernov2.ValueOperatorEquals,
			expected:       false,
		},
		{
			name:           "startsWith operator - match",
			value:          "internal-registry.company.com/app:latest",
			exemptedValues: []string{"internal-registry.company.com"},
			operator:       kyvernov2.ValueOperatorStartsWith,
			expected:       true,
		},
		{
			name:           "contains operator - match",
			value:          "my-app-debug-build",
			exemptedValues: []string{"debug"},
			operator:       kyvernov2.ValueOperatorContains,
			expected:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := matchesValue(tc.value, tc.exemptedValues, tc.operator)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExtractValuesFromPath(t *testing.T) {
	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"environment": "debug",
					"app":         "test-app",
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "app-container",
						"image": "internal-registry.company.com/app:v1.0",
					},
				},
			},
		},
	}

	testCases := []struct {
		name     string
		path     string
		expected []string
	}{
		{
			name:     "simple metadata label path",
			path:     "metadata.labels.environment",
			expected: []string{"debug"},
		},
		{
			name:     "container images path",
			path:     "spec.containers[*].image",
			expected: []string{"internal-registry.company.com/app:v1.0"},
		},
		{
			name:     "non-existent path",
			path:     "metadata.labels.nonexistent",
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := extractValuesFromPath(tc.path, resource)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExceptionHelperMethods(t *testing.T) {
	exception := kyvernov2.Exception{
		PolicyName: "test-policy",
		RuleNames:  []string{"test-rule"},
		Images: []kyvernov2.ImageException{
			{ImageReferences: []string{"registry.com/*"}},
		},
		Values: []kyvernov2.ValueException{
			{
				Path:   "metadata.labels.env",
				Values: []string{"debug"},
			},
		},
		ReportAs: &[]kyvernov2.ExceptionReportMode{kyvernov2.ExceptionReportWarn}[0],
	}

	assert.True(t, exception.HasImageExceptions())
	assert.True(t, exception.HasValueExceptions())
	assert.True(t, exception.IsFinegrained())
	assert.Equal(t, kyvernov2.ExceptionReportWarn, exception.GetReportMode())

	emptyException := kyvernov2.Exception{
		PolicyName: "test-policy",
		RuleNames:  []string{"test-rule"},
	}

	assert.False(t, emptyException.HasImageExceptions())
	assert.False(t, emptyException.HasValueExceptions())
	assert.False(t, emptyException.IsFinegrained())
	assert.Equal(t, kyvernov2.ExceptionReportSkip, emptyException.GetReportMode())
}
