package yaml

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsEmptyDocument(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want bool
	}{
		// Trivially empty
		{"empty string", "", true},
		{"blank lines only", "\n\n\n", true},
		// Comments
		{"comment only", "# just a comment", true},
		{"multiple comments", "# one\n# two\n# three", true},
		// doc-start marker ---
		{"bare doc-start", "---", true},
		{"doc-start with trailing space", "---   ", true},
		{"doc-start with inline comment", "--- # source: chart/template.yaml", true},
		{"doc-start with tab then comment", "---\t# tabbed", true},
		// doc-end marker ...
		{"bare doc-end", "...", true},
		{"doc-end with trailing space", "...   ", true},
		{"doc-end with inline comment", "... # done", true},
		// Mixed markers and comments, no real content
		{"separator sandwich with comment", "---\n# Source: helm-chart/crds.yaml\n---", true},
		{"doc-start and doc-end", "---\n...", true},
		{"doc-start comment doc-end", "---\n# comment\n...", true},
		// Real content — must NOT be empty
		{"real content", "apiVersion: v1\nkind: Pod", false},
		{"marker then content", "---\napiVersion: v1", false},
		{"content then doc-end", "apiVersion: v1\n...", false},
		// Marker look-alikes that are NOT document markers
		{"four dashes", "----", false},
		{"four dots", "....", false},
		{"doc-start glued to word", "---foo", false},
		{"doc-end glued to word", "...bar", false},
		{"doc-start glued to comment", "---#foo", false},
		{"doc-end glued to comment", "...#bar", false},
		{"doc-start with non-comment text", "--- not a comment", false},
		{"doc-end with non-comment text", "... not a comment", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsEmptyDocument([]byte(tt.doc)))
		})
	}
}
