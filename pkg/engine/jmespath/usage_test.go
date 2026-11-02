package jmespath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUsedFunctionNames(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want []string
	}{
		{
			name: "no function call",
			expr: "deployments[*].name",
			want: nil,
		},
		{
			name: "single known function",
			expr: "add(replicas, 1)",
			want: []string{"add"},
		},
		{
			name: "multiple known functions deduped and sorted",
			expr: "to_upper(trim(to_upper(name)))",
			want: []string{"to_upper", "trim"},
		},
		{
			name: "unknown identifier call is ignored",
			expr: "not_a_real_function(name)",
			want: nil,
		},
		{
			name: "builtin jmespath function is not in the known custom registry",
			expr: "length(items)",
			want: nil,
		},
		{
			name: "function-shaped text inside a string literal is not a call",
			expr: "'add(foo)'",
			want: nil,
		},
		{
			name: "function-shaped text inside a double-quoted literal is not a call",
			expr: `"add(foo)"`,
			want: nil,
		},
		{
			name: "function-shaped text inside a backtick JSON literal is not a call",
			expr: "`{\"k\": \"add(foo)\"}`",
			want: nil,
		},
		{
			name: "real call alongside a literal containing function-shaped text",
			expr: "add('not_a_call(x)', replicas)",
			want: []string{"add"},
		},
		{
			name: "escaped quote inside a literal does not end it early",
			expr: `'it\'s not add(foo)'`,
			want: nil,
		},
		{
			name: "real call survives after an escaped-quote literal",
			expr: `trim('it\'s a literal') + add(replicas, 1)`,
			want: []string{"add", "trim"},
		},
		{
			name: "unterminated literal blanks the remainder instead of panicking",
			expr: "add(replicas) + 'unterminated add(foo)",
			want: []string{"add"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UsedFunctionNames(tt.expr)
			assert.Equal(t, tt.want, got)
		})
	}
}
