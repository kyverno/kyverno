package yaml

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsEmptyDocument(t *testing.T) {
	tests := []struct {
		name     string
		document document
		want     bool
	}{{
		name:     "nil document",
		document: nil,
		want:     true,
	}, {
		name:     "empty string",
		document: []byte(""),
		want:     true,
	}, {
		name:     "only whitespace",
		document: []byte("   \n  \t  \n  "),
		want:     true,
	}, {
		name:     "only comments",
		document: []byte("# this is a comment\n# another comment"),
		want:     true,
	}, {
		name:     "comments with whitespace",
		document: []byte("  # indented comment\n\n  # another\n"),
		want:     true,
	}, {
		name:     "single key-value",
		document: []byte("key: value"),
		want:     false,
	}, {
		name:     "yaml with comments and content",
		document: []byte("# comment\nkey: value\n# trailing comment"),
		want:     false,
	}, {
		name:     "multi-line yaml",
		document: []byte("apiVersion: v1\nkind: Pod\nmetadata:\n  name: test"),
		want:     false,
	}, {
		name:     "document separator only",
		document: []byte("---"),
		want:     false,
	}, {
		name:     "empty lines only",
		document: []byte("\n\n\n"),
		want:     true,
	}, {
		name:     "content after comments",
		document: []byte("# comment\n\nenabled: true"),
		want:     false,
	}, {
		name:     "single non-comment character",
		document: []byte("a"),
		want:     false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsEmptyDocument(tt.document)
			assert.Equal(t, tt.want, got)
		})
	}
}
