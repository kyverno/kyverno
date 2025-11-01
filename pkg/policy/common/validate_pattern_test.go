package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateStringPattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid simple string",
			pattern:     "hello",
			expectError: false,
		},
		{
			name:        "Valid range operator",
			pattern:     "1-10",
			expectError: false,
		},
		{
			name:        "Valid range with units",
			pattern:     "1Mi-10Mi",
			expectError: false,
		},
		{
			name:        "Valid negative number",
			pattern:     "-5",
			expectError: false,
		},
		{
			name:        "Valid not-in-range operator",
			pattern:     "1!-10",
			expectError: false,
		},
		{
			name:        "Invalid not-in-range syntax",
			pattern:     "1!-",
			expectError: true,
			errorMsg:    "invalid operator syntax in pattern '1!-': !- requires range format",
		},
		{
			name:        "Valid comparison operators",
			pattern:     ">=5",
			expectError: false,
		},
		{
			name:        "Valid less than operator",
			pattern:     "<10",
			expectError: false,
		},
		{
			name:        "Valid not equal operator",
			pattern:     "!disabled",
			expectError: false,
		},
		{
			name:        "Valid string with dash in content",
			pattern:     "test-value",
			expectError: false,
		},
		{
			name:        "Valid string with multiple dashes",
			pattern:     "my-app-name",
			expectError: false,
		},
		{
			name:        "Valid domain-like string",
			pattern:     "example.com/nginx",
			expectError: false,
		},
		{
			name:        "Invalid numeric range with trailing dash",
			pattern:     "5-",
			expectError: true,
			errorMsg:    "invalid range operator syntax in pattern '5-'",
		},
		{
			name:        "Invalid memory range with trailing dash",
			pattern:     "1Gi-",
			expectError: true,
			errorMsg:    "invalid range operator syntax in pattern '1Gi-'",
		},
		{
			name:        "Valid namespace selector string",
			pattern:     "production-ns",
			expectError: false,
		},
		{
			name:        "Valid image name with registry",
			pattern:     "prod.example.com/nginx",
			expectError: false,
		},
		{
			name:        "Valid string with dots and dashes",
			pattern:     "staging.example.com/my-app",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := validateStringPattern(tt.pattern, "test.path")
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Empty(t, path)
			}
		})
	}
}

func TestValidatePattern_StringOperatorValidation(t *testing.T) {
	// Test the integration with ValidatePattern function
	testCases := []struct {
		name        string
		element     interface{}
		expectError bool
	}{
		{
			name:        "Valid string pattern",
			element:     ">=5",
			expectError: false,
		},
		{
			name:        "Invalid operator syntax",
			element:     "1!-",
			expectError: true,
		},
		{
			name:        "Non-string element",
			element:     123,
			expectError: false,
		},
		{
			name:        "Boolean element",
			element:     true,
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path, err := ValidatePattern(tc.element, "test.path", nil)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Empty(t, path)
			}
		})
	}
}
