package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildStatementMap(t *testing.T) {
	tests := []struct {
		name           string
		statements     []map[string]any
		wantTypes      []string
		wantErr        bool
		wantErrMessage string
	}{{
		name: "groups statements by type",
		statements: []map[string]any{
			{"type": "https://slsa.dev/provenance/v0.2"},
			{"type": "https://example.com/sbom"},
			{"type": "https://slsa.dev/provenance/v0.2"},
		},
		wantTypes: []string{
			"https://slsa.dev/provenance/v0.2",
			"https://example.com/sbom",
			"https://slsa.dev/provenance/v0.2",
		},
	}, {
		name:       "no statements",
		statements: nil,
		wantTypes:  []string{},
	}, {
		// The notary path builds these maps from a registry manifest, which only
		// substitutes the artifact type when 'type' is absent. A present but
		// non-string value used to reach an unchecked assertion and panic.
		name: "numeric type is rejected, not asserted",
		statements: []map[string]any{
			{"type": float64(1)},
		},
		wantErr:        true,
		wantErrMessage: "statement[0] 'type' found to be of the type float64. The type is expected to be a string",
	}, {
		name: "missing type is rejected",
		statements: []map[string]any{
			{"predicate": map[string]any{}},
		},
		wantErr:        true,
		wantErrMessage: "statement[0] 'type' found to be of the type <nil>. The type is expected to be a string",
	}, {
		name: "the offending index is reported",
		statements: []map[string]any{
			{"type": "https://example.com/sbom"},
			{"type": map[string]any{"nested": true}},
		},
		wantErr:        true,
		wantErrMessage: "statement[1] 'type' found to be of the type map[string]interface {}. The type is expected to be a string",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, types, err := buildStatementMap(tt.statements)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.wantErrMessage, err.Error())
				assert.Nil(t, results)
				assert.Nil(t, types)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantTypes, types)
			for _, s := range tt.statements {
				predicateType := s["type"].(string)
				assert.Contains(t, results[predicateType], s)
			}
		})
	}
}
