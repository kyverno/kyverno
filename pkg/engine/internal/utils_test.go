package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildStatementMapRejectsMalformedType is the harden-type-handling
// regression: remote/attacker-controlled statement JSON with a non-string or
// missing "type" field must produce an error, never panic.
func TestBuildStatementMapRejectsMalformedType(t *testing.T) {
	tests := []struct {
		name       string
		statements []map[string]any
	}{
		{
			name: "type is a number",
			statements: []map[string]any{
				{"type": 1337},
			},
		},
		{
			name: "type is missing",
			statements: []map[string]any{
				{"predicate": map[string]any{}},
			},
		},
		{
			name: "type is a nested object",
			statements: []map[string]any{
				{"type": map[string]any{"nested": true}},
			},
		},
		{
			name: "type is nil",
			statements: []map[string]any{
				{"type": nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotPanics(t, func() {
				_, _, err := buildStatementMap(tt.statements)
				assert.Error(t, err)
			})
		})
	}
}

// TestBuildStatementMapGroupsByType confirms the well-formed path still
// groups statements by predicate type correctly, so the hardening above
// doesn't change behavior for valid input.
func TestBuildStatementMapGroupsByType(t *testing.T) {
	statements := []map[string]any{
		{"type": "type-a", "value": 1},
		{"type": "type-b", "value": 2},
		{"type": "type-a", "value": 3},
	}

	results, types, err := buildStatementMap(statements)
	require.NoError(t, err)
	assert.Equal(t, []string{"type-a", "type-b", "type-a"}, types)
	assert.Len(t, results["type-a"], 2)
	assert.Len(t, results["type-b"], 1)
}
