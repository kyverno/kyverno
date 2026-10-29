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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UsedFunctionNames(tt.expr)
			assert.Equal(t, tt.want, got)
		})
	}
}
