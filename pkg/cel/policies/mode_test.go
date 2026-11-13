package policies

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/require"
)

func TestEvaluationModeMatchExpressions(t *testing.T) {
	policyCases := []struct {
		name   string
		object map[string]any
		want   bool
	}{
		{name: "absent spec", object: map[string]any{}, want: true},
		{name: "absent evaluation", object: map[string]any{"spec": map[string]any{}}, want: true},
		{name: "absent mode", object: map[string]any{"spec": map[string]any{"evaluation": map[string]any{}}}, want: true},
		{name: "empty", object: map[string]any{"spec": map[string]any{"evaluation": map[string]any{"mode": ""}}}, want: true},
		{name: "kubernetes", object: map[string]any{"spec": map[string]any{"evaluation": map[string]any{"mode": "Kubernetes"}}}, want: true},
		{name: "json", object: map[string]any{"spec": map[string]any{"evaluation": map[string]any{"mode": "JSON"}}}, want: true},
		{name: "envoy", object: map[string]any{"spec": map[string]any{"evaluation": map[string]any{"mode": "Envoy"}}}, want: false},
	}
	exceptionCases := []struct {
		name   string
		object map[string]any
		want   bool
	}{
		{name: "absent spec", object: map[string]any{}, want: true},
		{name: "absent mode", object: map[string]any{"spec": map[string]any{}}, want: true},
		{name: "empty", object: map[string]any{"spec": map[string]any{"evaluationMode": ""}}, want: true},
		{name: "kubernetes", object: map[string]any{"spec": map[string]any{"evaluationMode": "Kubernetes"}}, want: true},
		{name: "json", object: map[string]any{"spec": map[string]any{"evaluationMode": "JSON"}}, want: true},
		{name: "envoy", object: map[string]any{"spec": map[string]any{"evaluationMode": "Envoy"}}, want: false},
	}

	t.Run("policy", func(t *testing.T) {
		for _, tt := range policyCases {
			t.Run(tt.name, func(t *testing.T) {
				got := evalMatch(t, PolicyEvaluationModeExpression, tt.object)
				require.Equal(t, tt.want, got)
			})
		}
	})
	t.Run("exception", func(t *testing.T) {
		for _, tt := range exceptionCases {
			t.Run(tt.name, func(t *testing.T) {
				got := evalMatch(t, ExceptionEvaluationModeExpression, tt.object)
				require.Equal(t, tt.want, got)
			})
		}
	})
}

func evalMatch(t *testing.T, expression string, object map[string]any) bool {
	t.Helper()
	env, err := cel.NewEnv(cel.Variable("object", cel.DynType))
	require.NoError(t, err)
	ast, iss := env.Compile(expression)
	require.NoError(t, iss.Err())
	prog, err := env.Program(ast)
	require.NoError(t, err)
	out, _, err := prog.Eval(map[string]any{"object": object})
	require.NoError(t, err)
	value, ok := out.Value().(bool)
	require.True(t, ok)
	return value
}
