package utils

import (
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/ext/wildcard"
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

func TestWildcardMatching(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		image    string
		expected bool
	}{
		{
			name:     "exact match",
			pattern:  "nginx:latest",
			image:    "nginx:latest",
			expected: true,
		},
		{
			name:     "wildcard match",
			pattern:  "nginx:*",
			image:    "nginx:1.20.1",
			expected: true,
		},
		{
			name:     "no match",
			pattern:  "nginx:*",
			image:    "redis:latest",
			expected: false,
		},
		{
			name:     "wildcard registry match",
			pattern:  "registry.com/*",
			image:    "registry.com/app:latest",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Using the wildcard.Match function directly since that's what's used internally
			result := wildcard.Match(tt.pattern, tt.image)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchesValueWithDifferentOperators(t *testing.T) {
	tests := []struct {
		name           string
		value          string
		exemptedValues []string
		operator       kyvernov2.ValueOperator
		expected       bool
	}{
		{
			name:           "in operator - match",
			value:          "development",
			exemptedValues: []string{"development", "testing", "staging"},
			operator:       kyvernov2.ValueOperatorIn,
			expected:       true,
		},
		{
			name:           "in operator - no match",
			value:          "production",
			exemptedValues: []string{"development", "testing", "staging"},
			operator:       kyvernov2.ValueOperatorIn,
			expected:       false,
		},
		{
			name:           "endsWith operator - match",
			value:          "myapp-development",
			exemptedValues: []string{"-development", "-testing"},
			operator:       kyvernov2.ValueOperatorEndsWith,
			expected:       true,
		},
		{
			name:           "endsWith operator - no match",
			value:          "myapp-production",
			exemptedValues: []string{"-development", "-testing"},
			operator:       kyvernov2.ValueOperatorEndsWith,
			expected:       false,
		},
		{
			name:           "default operator (equals)",
			value:          "debug",
			exemptedValues: []string{"debug", "test"},
			operator:       "", // Empty should default to equals
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesValue(tt.value, tt.exemptedValues, tt.operator)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractValuesFromPathEdgeCases(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"app": "test-app",
					"env": "", // Empty string value
				},
				"annotations": map[string]interface{}{
					"key1": "value1",
				},
			},
			"spec": map[string]interface{}{
				"replicas": 3, // Non-string value
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "container1",
						"image": "nginx:latest",
					},
					map[string]interface{}{
						"name":  "container2",
						"image": "alpine:latest",
					},
				},
			},
		},
	}

	tests := []struct {
		name        string
		path        string
		expected    []string
		expectError bool
	}{
		{
			name:        "empty string value",
			path:        "metadata.labels.env",
			expected:    []string{""},
			expectError: false,
		},
		{
			name:        "non-string value causes error",
			path:        "spec.replicas",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "deeply nested non-existent path",
			path:        "spec.template.spec.containers[*].resources.limits.memory",
			expected:    nil,
			expectError: false,
		},
		{
			name:        "invalid array syntax",
			path:        "spec.containers[invalid].name",
			expected:    nil,
			expectError: false,
		},
		{
			name:        "array with out of bounds index",
			path:        "spec.containers[10].name",
			expected:    nil,
			expectError: false,
		},
		{
			name:        "valid annotation path",
			path:        "metadata.annotations.key1",
			expected:    []string{"value1"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractValuesFromPath(tt.path, *resource)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
