package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScan(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []segment
		wantErr string
	}{{
		name:  "plain literal",
		input: "hello",
		want:  []segment{{literal: "hello"}},
	}, {
		name:  "whole placeholder",
		input: "(( variables.name ))",
		want:  []segment{{expression: "variables.name", isExpr: true}},
	}, {
		name:  "embedded placeholder",
		input: "prefix-(( variables.name ))-suffix",
		want: []segment{
			{literal: "prefix-"},
			{expression: "variables.name", isExpr: true},
			{literal: "-suffix"},
		},
	}, {
		name:  "multiple placeholders",
		input: "(( a ))/(( b ))",
		want: []segment{
			{expression: "a", isExpr: true},
			{literal: "/"},
			{expression: "b", isExpr: true},
		},
	}, {
		name:  "nested function calls",
		input: "(( string(f(x)) ))",
		want:  []segment{{expression: "string(f(x))", isExpr: true}},
	}, {
		name:  "parenthesis in string literal",
		input: `(( ":)".size() ))`,
		want:  []segment{{expression: `":)".size()`, isExpr: true}},
	}, {
		name:  "double parenthesis in expression",
		input: `(( string(size(x)) ))`,
		want:  []segment{{expression: "string(size(x))", isExpr: true}},
	}, {
		name:  "escaped placeholder",
		input: `\(( not a placeholder ))`,
		want:  []segment{{literal: "(( not a placeholder ))"}},
	}, {
		name:  "escape followed by placeholder",
		input: `\((-(( x ))`,
		want: []segment{
			{literal: "((-"},
			{expression: "x", isExpr: true},
		},
	}, {
		name:    "unterminated placeholder",
		input:   "(( variables.name",
		wantErr: "unterminated placeholder",
	}, {
		name:    "unbalanced parenthesis",
		input:   "(( a ) b ))x",
		wantErr: "unbalanced parenthesis",
	}, {
		name:    "empty placeholder",
		input:   "(( ))",
		wantErr: "empty placeholder",
	}, {
		name:    "unterminated string literal",
		input:   `(( "abc ))`,
		wantErr: "unterminated string literal",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scan(tt.input)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestContainsPlaceholder(t *testing.T) {
	assert.True(t, containsPlaceholder("(( x ))"))
	assert.True(t, containsPlaceholder("a(( x ))b"))
	assert.False(t, containsPlaceholder("plain"))
	assert.False(t, containsPlaceholder(`\(( escaped ))`))
	assert.True(t, containsPlaceholder(`\((-(( x ))`))
}
