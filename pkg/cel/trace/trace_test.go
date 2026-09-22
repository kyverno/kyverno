package trace

import (
	"context"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/stretchr/testify/require"
)

// buildTrackedProgram compiles expr with cel.OptTrackState on, bypassing the real policy
// compiler (which doesn't have a trace on/off switch wired up yet) so the trace mechanism can
// be exercised directly, the same way it was first proven to work.
func buildTrackedProgram(t *testing.T, expr string) (cel.Program, *cel.Ast) {
	t.Helper()
	env, err := compiler.NewBaseEnv()
	require.NoError(t, err)
	env, err = env.Extend(cel.Variable("object", cel.DynType))
	require.NoError(t, err)

	ast, iss := env.Compile(expr)
	require.NoError(t, iss.Err())

	prg, err := env.Program(ast, cel.EvalOptions(cel.OptTrackState))
	require.NoError(t, err)
	return prg, ast
}

func TestBuild(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		object map[string]any
	}{
		{
			name: "has app label",
			expr: "has(object.metadata.labels) && 'app' in object.metadata.labels",
			object: map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{"app": "nginx"},
				},
			},
		},
		{
			name: "missing app label",
			expr: "has(object.metadata.labels) && 'app' in object.metadata.labels",
			object: map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{"team": "platform"},
				},
			},
		},
		{
			name: "missing key error",
			expr: "object.metadata.labels.team == 'platform'",
			object: map[string]any{
				"metadata": map[string]any{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg, ast := buildTrackedProgram(t, tt.expr)
			out, details, err := prg.ContextEval(context.TODO(), map[string]any{"object": tt.object})
			// a failing sub-expression can surface as either a top-level eval error or an
			// error-typed result depending on where it happens; Build handles both, so we
			// don't fail the test on err here.
			_ = err

			et := Build(tt.expr, ast, out, details)

			t.Logf("expression: %s", et.Source)
			for _, n := range et.Nodes {
				if n.Error != "" {
					t.Logf("  %-45s ->  ERROR: %s", n.Expression, n.Error)
				} else {
					t.Logf("  %-45s ->  %s", n.Expression, n.Value)
				}
			}
			t.Logf("  result: %s", et.Result)
		})
	}
}

// TestBuild_StableOrder runs the same expression many times and checks the node order never
// changes -- this is what the map-order fix (collecting nodes during PreOrderVisit instead of
// from state.IDs()) is actually guaranteeing.
func TestBuild_StableOrder(t *testing.T) {
	expr := "has(object.metadata.labels) && 'app' in object.metadata.labels"
	prg, ast := buildTrackedProgram(t, expr)
	object := map[string]any{
		"metadata": map[string]any{"labels": map[string]any{"app": "nginx"}},
	}

	var want []string
	for i := 0; i < 200; i++ {
		out, details, err := prg.ContextEval(context.TODO(), map[string]any{"object": object})
		require.NoError(t, err)
		et := Build(expr, ast, out, details)

		var got []string
		for _, n := range et.Nodes {
			got = append(got, n.Expression)
		}
		if want == nil {
			want = got
			continue
		}
		require.Equal(t, want, got, "node order changed on run %d", i)
	}
}
