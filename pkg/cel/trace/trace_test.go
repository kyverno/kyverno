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
		name       string
		expr       string
		object     map[string]any
		wantResult string
		wantNodes  []NodeTrace
	}{
		{
			name: "has app label",
			expr: "has(object.metadata.labels) && 'app' in object.metadata.labels",
			object: map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{"app": "nginx"},
				},
			},
			wantResult: "true",
			wantNodes: []NodeTrace{
				{Expression: `has(object.metadata.labels) && "app" in object.metadata.labels`, Value: "true"},
				{Expression: "has(object.metadata.labels)", Value: "true"},
				{Expression: "object.metadata", Value: "map[labels:map[app:nginx]]"},
				{Expression: `"app" in object.metadata.labels`, Value: "true"},
				{Expression: "object.metadata.labels", Value: "map[app:nginx]"},
				{Expression: "object.metadata", Value: "map[labels:map[app:nginx]]"},
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
			wantResult: "false",
			wantNodes: []NodeTrace{
				{Expression: `has(object.metadata.labels) && "app" in object.metadata.labels`, Value: "false"},
				{Expression: "has(object.metadata.labels)", Value: "true"},
				{Expression: "object.metadata", Value: "map[labels:map[team:platform]]"},
				{Expression: `"app" in object.metadata.labels`, Value: "false"},
				{Expression: "object.metadata.labels", Value: "map[team:platform]"},
				{Expression: "object.metadata", Value: "map[labels:map[team:platform]]"},
			},
		},
		{
			// the failing lookup and everything built on it must land in Error, never in Value,
			// while the parent that did resolve keeps its value
			name: "missing key error",
			expr: "object.metadata.labels.team == 'platform'",
			object: map[string]any{
				"metadata": map[string]any{},
			},
			wantResult: "no such key: labels",
			wantNodes: []NodeTrace{
				{Expression: `object.metadata.labels.team == "platform"`, Error: "no such key: labels"},
				{Expression: "object.metadata.labels.team", Error: "no such key: labels"},
				{Expression: "object.metadata.labels", Error: "no such key: labels"},
				{Expression: "object.metadata", Value: "map[]"},
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

			assert.Equal(t, tt.expr, et.Source)
			assert.Equal(t, tt.wantResult, et.Result)
			assert.Equal(t, tt.wantNodes, et.Nodes)
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
	// the list being looped over runs once and is kept; the loop body is left out (see
	// TestBuild_LoopBodiesAreOmitted)
	assert.Contains(t, nodeTexts(et), "object.spec.containers")
	assert.NotContains(t, nodeTexts(et), "has(c.resources)")
	assert.True(t, et.LoopValuesOmitted)
}

// TestBuild_LoopBodiesAreOmitted is the review repro: cel-go keeps one value per node, from its
// last run, so inside a loop a node shows the last item's value even when another item decided
// the result. With two containers where the second fails, the body used to show the second's
// image next to the first's name. Body nodes must be left out, and the omission flagged.
func TestBuild_LoopBodiesAreOmitted(t *testing.T) {
	object := map[string]any{"spec": map[string]any{"containers": []any{
		map[string]any{"name": "app", "image": "eu.foo.io/app:1.0", "resources": map[string]any{}},
		map[string]any{"name": "sidecar", "image": "docker.io/envoyproxy/envoy:v1.30"},
	}}}
	tests := []struct {
		name string
		expr string
		// kept are the nodes that run once and must still be traced
		kept []string
	}{{
		name: "all",
		expr: "object.spec.containers.all(c, c.image.startsWith('eu.foo.io/') && c.name != 'forbidden' && has(c.resources))",
		kept: []string{"object.spec.containers", "object.spec"},
	}, {
		name: "exists",
		expr: "object.spec.containers.exists(c, c.name == 'sidecar')",
		kept: []string{"object.spec.containers", "object.spec"},
	}, {
		// nodes that contain a loop (the size() and == here) are not traced either: without
		// macro call tracking in the policy environment the CEL unparser cannot render them
		name: "map, then a check on its result outside the loop",
		expr: "object.spec.containers.map(c, c.name).size() == 2",
		kept: []string{"object.spec.containers", "object.spec"},
	}, {
		name: "loop over the result of another loop",
		expr: "object.spec.containers.filter(c, has(c.resources)).all(c, c.image.startsWith('eu.foo.io/'))",
		kept: []string{"object.spec.containers", "object.spec"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg, ast := buildTrackedProgram(t, tt.expr)
			out, details, err := prg.ContextEval(context.TODO(), map[string]any{"object": object})
			require.NoError(t, err)
			et := Build(tt.expr, ast, out, details)

			texts := nodeTexts(et)
			for _, text := range texts {
				assert.NotRegexp(t, `\bc\.`, text, "a node from inside a loop body must not be traced: %q", text)
			}
			assert.Equal(t, tt.kept, texts, "only the nodes that run once are traced")
			assert.True(t, et.LoopValuesOmitted)
		})
	}
}

func TestBuild_NoLoopNothingOmitted(t *testing.T) {
	prg, ast := buildTrackedProgram(t, "object.metadata.name == 'x'")
	out, details, err := prg.ContextEval(context.TODO(), map[string]any{"object": map[string]any{"metadata": map[string]any{"name": "x"}}})
	require.NoError(t, err)
	assert.False(t, Build("object.metadata.name == 'x'", ast, out, details).LoopValuesOmitted)
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

// TestBuild_CallReceiverIdentifiersAreOmitted checks that a bare identifier used as a call
// receiver (in real policies a library handle like resource or generator, whose value renders
// as a pointer) is left out, while the call itself and data paths are still traced.
func TestBuild_CallReceiverIdentifiersAreOmitted(t *testing.T) {
	env, err := compiler.NewBaseEnv()
	require.NoError(t, err)
	env, err = env.Extend(cel.Variable("name", cel.StringType), cel.Variable("object", cel.DynType))
	require.NoError(t, err)
	expr := "name.startsWith('x') && object.metadata.name.startsWith('x')"
	ast, iss := env.Compile(expr)
	require.NoError(t, iss.Err())
	prg, err := env.Program(ast, cel.EvalOptions(cel.OptTrackState))
	require.NoError(t, err)

	out, details, err := prg.ContextEval(context.TODO(), map[string]any{
		"name":   "xyz",
		"object": map[string]any{"metadata": map[string]any{"name": "xyz"}},
	})
	require.NoError(t, err)
	texts := nodeTexts(Build(expr, ast, out, details))

	assert.NotContains(t, texts, "name", "a bare call receiver must not be traced")
	assert.Contains(t, texts, `name.startsWith("x")`, "the call itself is still traced")
	assert.Contains(t, texts, `object.metadata.name.startsWith("x")`)
	assert.Contains(t, texts, "object.metadata.name", "a qualified receiver is still traced")
}

// TestBuild_LongExpressionsStayOnOneLine checks that node text never contains a newline: the CEL
// unparser wraps long &&/|| expressions, which would break the one-line-per-node breakdown.
func TestBuild_LongExpressionsStayOnOneLine(t *testing.T) {
	expr := "object.metadata.name.startsWith('a-very-long-prefix-to-force-wrapping') && object.metadata.namespace.endsWith('another-long-suffix-value') || object.metadata.name == 'x\\ny'"
	prg, ast := buildTrackedProgram(t, expr)
	out, details, err := prg.ContextEval(context.TODO(), map[string]any{
		"object": map[string]any{"metadata": map[string]any{"name": "n", "namespace": "ns"}},
	})
	require.NoError(t, err)
	et := Build(expr, ast, out, details)
	require.NotEmpty(t, et.Nodes)
	for _, n := range et.Nodes {
		assert.NotContains(t, n.Expression, "\n", "node text must stay on one line: %q", n.Expression)
	}
	assert.Contains(t, nodeTexts(et), `object.metadata.name == "x\ny"`, "an escaped newline inside a string literal is kept as written")
}
