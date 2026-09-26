package trace

import (
	"context"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/stretchr/testify/assert"
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

func nodeTexts(et ExpressionTrace) []string {
	texts := make([]string, 0, len(et.Nodes))
	for _, n := range et.Nodes {
		texts = append(texts, n.Expression)
	}
	return texts
}

func TestBuild_MacroInternalsAndLiteralsAreOmitted(t *testing.T) {
	expr := "object.spec.containers.all(c, has(c.resources) && 'memory' in c.resources.requests)"
	prg, ast := buildTrackedProgram(t, expr)
	object := map[string]any{"spec": map[string]any{"containers": []any{map[string]any{"name": "a"}}}}

	out, details, err := prg.ContextEval(context.TODO(), map[string]any{"object": object})
	require.NoError(t, err)
	et := Build(expr, ast, out, details)

	assert.Equal(t, "false", et.Result, "the whole comprehension's outcome is carried by Result")
	for _, text := range nodeTexts(et) {
		assert.NotContains(t, text, "@", "macro-internal nodes such as @result must not be traced")
		assert.NotEqual(t, "true", text, "literal nodes must not be traced")
		assert.NotEqual(t, `"memory"`, text, "literal nodes must not be traced")
	}
	assert.Contains(t, nodeTexts(et), "has(c.resources)", "the meaningful sub-expression is kept")
}

func TestBuild_ShortCircuitedNodesAreOmitted(t *testing.T) {
	// the right-hand side never runs, so it has no value to trace
	expr := "false && object.metadata.labels.team == 'x'"
	prg, ast := buildTrackedProgram(t, expr)

	out, details, err := prg.ContextEval(context.TODO(), map[string]any{"object": map[string]any{}})
	require.NoError(t, err)
	et := Build(expr, ast, out, details)

	assert.Equal(t, "false", et.Result)
	texts := nodeTexts(et)
	// the whole expression is a traced node, but none of the right-hand operand's own
	// sub-expressions ever ran, so none of them may appear
	for _, unevaluated := range []string{"object.metadata", "object.metadata.labels", "object.metadata.labels.team"} {
		assert.NotContains(t, texts, unevaluated, "a short-circuited operand must not appear in the trace")
	}
}

func TestBuild_WithoutTrackingReturnsResultOnly(t *testing.T) {
	env, err := compiler.NewBaseEnv()
	require.NoError(t, err)
	ast, iss := env.Compile("1 + 1 == 2")
	require.NoError(t, iss.Err())
	prg, err := env.Program(ast)
	require.NoError(t, err)

	out, details, err := prg.ContextEval(context.TODO(), map[string]any{})
	require.NoError(t, err)
	require.Nil(t, details, "cel-go returns no details unless a tracking option was set")

	et := Build("1 + 1 == 2", ast, out, details)
	assert.Equal(t, "true", et.Result)
	assert.Empty(t, et.Nodes)
}

func TestBuild_CostOnlyTrackingDoesNotPanic(t *testing.T) {
	// details is non-nil here but carries no state; Build must fall back to the result alone
	env, err := compiler.NewBaseEnv()
	require.NoError(t, err)
	ast, iss := env.Compile("1 + 1 == 2")
	require.NoError(t, iss.Err())
	prg, err := env.Program(ast, cel.EvalOptions(cel.OptTrackCost))
	require.NoError(t, err)

	out, details, err := prg.ContextEval(context.TODO(), map[string]any{})
	require.NoError(t, err)
	require.NotNil(t, details)

	assert.NotPanics(t, func() {
		et := Build("1 + 1 == 2", ast, out, details)
		assert.Equal(t, "true", et.Result)
		assert.Empty(t, et.Nodes)
	})
}

func TestBuild_NilInputs(t *testing.T) {
	assert.NotPanics(t, func() {
		et := Build("expr", nil, nil, nil)
		assert.Equal(t, "expr", et.Source)
		assert.Empty(t, et.Result)
		assert.Empty(t, et.Nodes)
	})
}
