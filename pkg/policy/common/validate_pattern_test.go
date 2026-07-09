package common

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/engine/anchor"
	"github.com/stretchr/testify/assert"
)

func TestValidatePattern(t *testing.T) {
	allowAll := func(anchor.Anchor) bool { return true }
	allowNone := func(anchor.Anchor) bool { return false }

	tests := []struct {
		name        string
		pattern     interface{}
		isSupported func(anchor.Anchor) bool
		wantErr     bool
		errContains string
	}{
		{name: "string scalar", pattern: "value", isSupported: allowAll},
		{name: "float64 scalar", pattern: float64(1.5), isSupported: allowAll},
		{name: "int scalar", pattern: 1, isSupported: allowAll},
		{name: "int64 scalar", pattern: int64(2), isSupported: allowAll},
		{name: "bool scalar", pattern: true, isSupported: allowAll},
		{name: "nil value", pattern: nil, isSupported: allowAll},
		{
			name:        "unknown type at top level",
			pattern:     struct{}{},
			isSupported: allowAll,
			wantErr:     true,
			errContains: "unknown type",
		},
		{
			name:        "map with plain key recurses into value",
			pattern:     map[string]interface{}{"spec": "value"},
			isSupported: allowAll,
		},
		{
			name:        "map with unknown-type value propagates error",
			pattern:     map[string]interface{}{"spec": struct{}{}},
			isSupported: allowAll,
			wantErr:     true,
			errContains: "unknown type",
		},
		{
			name:        "supported existence anchor with non-empty list",
			pattern:     map[string]interface{}{"^(containers)": []interface{}{"nginx"}},
			isSupported: allowAll,
		},
		{
			name:        "existence anchor with non-list value",
			pattern:     map[string]interface{}{"^(containers)": "not-a-list"},
			isSupported: allowAll,
			wantErr:     true,
			errContains: "existence anchor should have value of type list",
		},
		{
			name:        "existence anchor with empty list",
			pattern:     map[string]interface{}{"^(containers)": []interface{}{}},
			isSupported: allowAll,
			wantErr:     true,
			errContains: "should have at least one value",
		},
		{
			name:        "anchor rejected when isSupported returns false",
			pattern:     map[string]interface{}{"^(containers)": []interface{}{"nginx"}},
			isSupported: allowNone,
			wantErr:     true,
			errContains: "unsupported anchor",
		},
		{
			name:        "anchor rejected when isSupported is nil",
			pattern:     map[string]interface{}{"^(containers)": []interface{}{"nginx"}},
			isSupported: nil,
			wantErr:     true,
			errContains: "unsupported anchor",
		},
		{
			name:        "array of valid elements",
			pattern:     []interface{}{"a", 1, true},
			isSupported: allowAll,
		},
		{
			name:        "array element error propagates with path",
			pattern:     []interface{}{struct{}{}},
			isSupported: allowAll,
			wantErr:     true,
			errContains: "unknown type",
		},
		{
			name: "nested map inside array is validated",
			pattern: []interface{}{
				map[string]interface{}{"^(containers)": []interface{}{}},
			},
			isSupported: allowAll,
			wantErr:     true,
			errContains: "should have at least one value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := ValidatePattern(tt.pattern, "root", tt.isSupported)
			if tt.wantErr {
				assert.Error(t, err)
				assert.NotEmpty(t, path, "an error result should carry the offending path")
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Empty(t, path)
			}
		})
	}
}
