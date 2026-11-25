package autogen

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanBalancedBraces(t *testing.T) {
	tests := []struct {
		name      string
		content   string // string immediately following the initial '{'
		wantFound bool
		wantIndex int
	}{
		{
			name:      "simple balanced struct",
			content:   "spec: Object.spec{template: {}}}",
			wantFound: true,
			wantIndex: 31,
		},
		{
			name:      "nested constructor followed by another constructor suffix",
			content:   "spec: Object.spec{template: {}}} + Object{suffix: {}}",
			wantFound: true,
			wantIndex: 31,
		},
		{
			name:      "JSON inside string literal",
			content:   `spec: Object.spec{containers: [{env: [{value: "{\"k\": \"v\"}"}]}]}}`,
			wantFound: true,
			wantIndex: 67,
		},
		{
			name:      "regex with range quantifiers inside single quotes",
			content:   `spec: Object.spec{pattern: '^[a-z]{1,3}$'}}`,
			wantFound: true,
			wantIndex: 42,
		},
		{
			name:      "triple-quoted multiline string with braces",
			content:   "spec: Object.spec{script: \"\"\"if (x) { return true; }\"\"\"}}",
			wantFound: true,
			wantIndex: 56,
		},
		{
			name:      "escaped quotes inside string",
			content:   `spec: Object.spec{msg: "foo \"{\" bar"}}`,
			wantFound: true,
			wantIndex: 39,
		},
		{
			name:      "braces inside line comment",
			content:   "spec: Object.spec{\n// { comment brace\n}}",
			wantFound: true,
			wantIndex: 39,
		},
		{
			name:      "braces inside block comment",
			content:   "spec: Object.spec{/* { comment brace } */}}",
			wantFound: true,
			wantIndex: 42,
		},
		{
			name:      "raw string with braces",
			content:   `spec: Object.spec{raw: r"{\"k\": \"v\"}"}}`,
			wantFound: true,
			wantIndex: 41,
		},
		{
			name:      "backtick raw string with opening brace",
			content:   "spec: Object.spec{command: `foo { bar`}}",
			wantFound: true,
			wantIndex: 39,
		},
		{
			name:      "backtick raw string with closing brace",
			content:   "spec: Object.spec{command: `foo } bar`}}",
			wantFound: true,
			wantIndex: 39,
		},
		{
			name:      "backtick raw string with balanced JSON",
			content:   "spec: Object.spec{command: `{\"key\": \"value\"}`}}",
			wantFound: true,
			wantIndex: 46,
		},
		{
			name:      "unclosed brace returns not found",
			content:   "spec: Object.spec{",
			wantFound: false,
			wantIndex: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, found := ScanBalancedBraces(tt.content)
			assert.Equal(t, tt.wantFound, found)
			assert.Equal(t, tt.wantIndex, idx)
			if tt.wantFound {
				require.GreaterOrEqual(t, idx, 0)
				assert.Equal(t, byte('}'), tt.content[idx])
				// Ensure index matches the 31st byte in the suffix test
				if strings.Contains(tt.content, "suffix") {
					assert.Equal(t, "spec: Object.spec{template: {}}", tt.content[:idx])
				}
			}
		})
	}
}
