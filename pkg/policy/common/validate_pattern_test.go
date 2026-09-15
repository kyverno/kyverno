package common

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/engine/anchor"
	"github.com/stretchr/testify/assert"
)

func TestValidatePattern(t *testing.T) {
	tests := []struct {
		name          string
		pattern       interface{}
		path          string
		isSupported   func(anchor.Anchor) bool
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid string pattern",
			pattern:     "test",
			path:        "/",
			isSupported: nil,
			expectError: false,
		},
		{
			name:        "string pattern with operator",
			pattern:     ">= 10",
			path:        "/",
			isSupported: nil,
			expectError: false,
		},
		{
			name:        "valid int pattern",
			pattern:     42,
			path:        "/",
			isSupported: nil,
			expectError: false,
		},
		{
			name:        "valid int64 pattern",
			pattern:     int64(42),
			path:        "/",
			isSupported: nil,
			expectError: false,
		},
		{
			name:        "valid float64 pattern",
			pattern:     42.5,
			path:        "/",
			isSupported: nil,
			expectError: false,
		},
		{
			name:        "valid bool pattern",
			pattern:     true,
			path:        "/",
			isSupported: nil,
			expectError: false,
		},
		{
			name:        "valid nil pattern",
			pattern:     nil,
			path:        "/",
			isSupported: nil,
			expectError: false,
		},
		{
			name:        "valid map pattern",
			pattern:     map[string]interface{}{"key": "value"},
			path:        "/",
			isSupported: nil,
			expectError: false,
		},
		{
			name:        "valid array pattern",
			pattern:     []interface{}{"item1", "item2"},
			path:        "/",
			isSupported: nil,
			expectError: false,
		},
		{
			name:    "unsupported anchor",
			pattern: map[string]interface{}{"X(key)": "value"},
			path:    "/",
			isSupported: func(a anchor.Anchor) bool {
				return false
			},
			expectError:   true,
			errorContains: "unsupported anchor",
		},
		{
			name:          "unknown type",
			pattern:       struct{}{},
			path:          "/",
			isSupported:   nil,
			expectError:   true,
			errorContains: "unknown type",
		},
		{
			name: "existence anchor with non-list value",
			pattern: map[string]interface{}{
				"^(key)": "not-a-list",
			},
			path:        "/",
			isSupported: func(a anchor.Anchor) bool { return true },
			expectError: true,
		},
		{
			name: "existence anchor with empty list",
			pattern: map[string]interface{}{
				"^(key)": []interface{}{},
			},
			path:        "/",
			isSupported: func(a anchor.Anchor) bool { return true },
			expectError: true,
		},
		{
			name: "valid existence anchor",
			pattern: map[string]interface{}{
				"^(key)": []interface{}{"value1", "value2"},
			},
			path:        "/",
			isSupported: func(a anchor.Anchor) bool { return true },
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := ValidatePattern(tt.pattern, tt.path, tt.isSupported)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				// Path should contain the original path as a prefix
				assert.Contains(t, path, tt.path)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "", path)
			}
		})
	}
}

func TestValidateStringPattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		path        string
		expectError bool
	}{
		{
			name:        "simple string",
			pattern:     "hello",
			path:        "/",
			expectError: false,
		},
		{
			name:        "string with equal operator",
			pattern:     "=hello",
			path:        "/",
			expectError: false,
		},
		{
			name:        "string with greater than operator",
			pattern:     ">10",
			path:        "/",
			expectError: false,
		},
		{
			name:        "string with less than operator",
			pattern:     "<10",
			path:        "/",
			expectError: false,
		},
		{
			name:        "string with greater than or equal operator",
			pattern:     ">=10",
			path:        "/",
			expectError: false,
		},
		{
			name:        "string with less than or equal operator",
			pattern:     "<=10",
			path:        "/",
			expectError: false,
		},
		{
			name:        "string with not equal operator",
			pattern:     "!hello",
			path:        "/",
			expectError: false,
		},
		{
			name:        "string with in range operator",
			pattern:     "1-10",
			path:        "/",
			expectError: false,
		},
		{
			name:        "string with not in range operator",
			pattern:     "!1-10",
			path:        "/",
			expectError: false,
		},
		{
			name:        "empty string",
			pattern:     "",
			path:        "/",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := validateStringPattern(tt.pattern, tt.path)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "", path)
			}
		})
	}
}

func TestValidateNumericPattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     interface{}
		path        string
		expectError bool
	}{
		{
			name:        "int pattern",
			pattern:     42,
			path:        "/",
			expectError: false,
		},
		{
			name:        "int64 pattern",
			pattern:     int64(42),
			path:        "/",
			expectError: false,
		},
		{
			name:        "float64 pattern",
			pattern:     42.5,
			path:        "/",
			expectError: false,
		},
		{
			name:        "zero int",
			pattern:     0,
			path:        "/",
			expectError: false,
		},
		{
			name:        "negative int",
			pattern:     -42,
			path:        "/",
			expectError: false,
		},
		{
			name:        "negative float64",
			pattern:     -42.5,
			path:        "/",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := validateNumericPattern(tt.pattern, tt.path)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "", path)
			}
		})
	}
}

func TestValidateBoolPattern(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "bool pattern",
			path:        "/",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := validateBoolPattern(tt.path)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "", path)
			}
		})
	}
}

func TestValidateNilPattern(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "nil pattern",
			path:        "/",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := validateNilPattern(tt.path)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "", path)
			}
		})
	}
}
