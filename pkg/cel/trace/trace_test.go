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
// be exercised directly.
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

// evaluate runs expr against object with tracking on and returns what Build emits. Evaluation
// errors are not fatal here: a failing sub-expression is exactly what some cases exercise, and
// Build reports it through the trace.
func evaluate(t *testing.T, expr string, object map[string]any) ExpressionTrace {
	t.Helper()
	prg, ast := buildTrackedProgram(t, expr)
	out, details, _ := prg.ContextEval(context.TODO(), map[string]any{"object": object})
	return Build(expr, ast, out, details)
}

// rendered flattens the nodes to "expression -> value" (or "expression -> ERROR: message") so a
// whole trace can be asserted in one comparison, order included.
func rendered(et ExpressionTrace) []string {
	lines := make([]string, 0, len(et.Nodes))
	for _, n := range et.Nodes {
		if n.Error != "" {
			lines = append(lines, n.Expression+" -> ERROR: "+n.Error)
			continue
		}
		lines = append(lines, n.Expression+" -> "+n.Value)
	}
	return lines
}

func expressions(et ExpressionTrace) []string {
	texts := make([]string, 0, len(et.Nodes))
	for _, n := range et.Nodes {
		texts = append(texts, n.Expression)
	}
	return texts
}

func TestBuild(t *testing.T) {
	const hasApp = "has(object.metadata.labels) && 'app' in object.metadata.labels"

	tests := []struct {
		name       string
		expr       string
		object     map[string]any
		wantResult string
		wantNodes  []string
	}{
		{
			name: "label present",
			expr: hasApp,
			object: map[string]any{"metadata": map[string]any{
				"labels": map[string]any{"app": "nginx"},
			}},
			wantResult: "true",
			wantNodes: []string{
				`has(object.metadata.labels) && "app" in object.metadata.labels -> true`,
				`has(object.metadata.labels) -> true`,
				`object.metadata -> map[labels:map[app:nginx]]`,
				`"app" in object.metadata.labels -> true`,
				`object.metadata.labels -> map[app:nginx]`,
				`object.metadata -> map[labels:map[app:nginx]]`,
			},
		},
		{
			name: "label absent shows what the labels actually held",
			expr: hasApp,
			object: map[string]any{"metadata": map[string]any{
				"labels": map[string]any{"team": "platform"},
			}},
			wantResult: "false",
			wantNodes: []string{
				`has(object.metadata.labels) && "app" in object.metadata.labels -> false`,
				`has(object.metadata.labels) -> true`,
				`object.metadata -> map[labels:map[team:platform]]`,
				`"app" in object.metadata.labels -> false`,
				`object.metadata.labels -> map[team:platform]`,
				`object.metadata -> map[labels:map[team:platform]]`,
			},
		},
		{
			name:       "short circuit leaves the unevaluated operand out",
			expr:       hasApp,
			object:     map[string]any{"metadata": map[string]any{}},
			wantResult: "false",
			wantNodes: []string{
				`has(object.metadata.labels) && "app" in object.metadata.labels -> false`,
				`has(object.metadata.labels) -> false`,
				`object.metadata -> map[]`,
			},
		},
		{
			name:       "missing key lands in Error, not Value",
			expr:       "object.metadata.labels.team == 'platform'",
			object:     map[string]any{"metadata": map[string]any{}},
			wantResult: "no such key: labels",
			wantNodes: []string{
				`object.metadata.labels.team == "platform" -> ERROR: no such key: labels`,
				`object.metadata.labels.team -> ERROR: no such key: labels`,
				`object.metadata.labels -> ERROR: no such key: labels`,
				`object.metadata -> map[]`,
			},
		},
		{
			name:       "an @ inside a string literal does not make a node macro noise",
			expr:       "object.email.endsWith('@example.com')",
			object:     map[string]any{"email": "a@example.com"},
			wantResult: "true",
			wantNodes: []string{
				`object.email.endsWith("@example.com") -> true`,
				`object.email -> a@example.com`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			et := evaluate(t, tt.expr, tt.object)

			assert.Equal(t, tt.expr, et.Source)
			assert.Equal(t, tt.wantResult, et.Result)
			assert.Equal(t, tt.wantNodes, rendered(et))
		})
	}
}

func TestBuild_ErrorNodesNeverCarryAValue(t *testing.T) {
	et := evaluate(t, "object.metadata.labels.team == 'platform'", map[string]any{"metadata": map[string]any{}})

	var errored, valued int
	for _, n := range et.Nodes {
		if n.Error != "" {
			errored++
			assert.Empty(t, n.Value, "%q errored, so it must not also carry a value", n.Expression)
		} else {
			valued++
		}
	}
	assert.Equal(t, 3, errored)
	assert.Equal(t, 1, valued)
}

func TestBuild_MacroInternalsAndLiteralsAreOmitted(t *testing.T) {
	containers := map[string]any{"spec": map[string]any{"containers": []any{
		map[string]any{"name": "a", "image": "nginx:latest"},
		map[string]any{"name": "b", "image": "envoy:v1"},
	}}}

	tests := []struct {
		name       string
		expr       string
		wantResult string
		wantNodes  []string
	}{
		{
			// all() stops at the first false, so the one iteration that ran is the failing one
			name:       "all",
			expr:       "object.spec.containers.all(c, c.image.endsWith(':v1'))",
			wantResult: "false",
			wantNodes: []string{
				`object.spec.containers -> [map[image:nginx:latest name:a] map[image:envoy:v1 name:b]]`,
				`object.spec -> map[containers:[map[image:nginx:latest name:a] map[image:envoy:v1 name:b]]]`,
				`c.image.endsWith(":v1") -> false`,
				`c.image -> nginx:latest`,
			},
		},
		{
			// The loop body is one AST node evaluated once per element, and cel-go keeps a single
			// value per node id, so each iteration overwrites the previous one. exists() ran the
			// body for both containers, but only the last iteration (the one that returned true)
			// survives in the trace. This pins that limitation so a change to it is deliberate.
			name:       "exists keeps only the last iteration of the body",
			expr:       "object.spec.containers.exists(c, c.image.endsWith(':v1'))",
			wantResult: "true",
			wantNodes: []string{
				`object.spec.containers -> [map[image:nginx:latest name:a] map[image:envoy:v1 name:b]]`,
				`object.spec -> map[containers:[map[image:nginx:latest name:a] map[image:envoy:v1 name:b]]]`,
				`c.image.endsWith(":v1") -> true`,
				`c.image -> envoy:v1`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			et := evaluate(t, tt.expr, containers)

			assert.Equal(t, tt.wantResult, et.Result, "the comprehension's outcome is carried by Result")
			assert.Equal(t, tt.wantNodes, rendered(et))
			for _, text := range expressions(et) {
				assert.NotContains(t, text, "@result", "macro internals such as @result must not be traced")
				assert.NotContains(t, text, "@not_strictly_false", "macro internals must not be traced")
			}
		})
	}
}

func TestBuild_LiteralsAreOmitted(t *testing.T) {
	et := evaluate(t, "1 + 2 == 3", map[string]any{})

	assert.Equal(t, "true", et.Result)
	assert.Equal(t, []string{`1 + 2 == 3 -> true`, `1 + 2 -> 3`}, rendered(et))
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
	assert.Equal(t, "1 + 1 == 2", et.Source)
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

// TestBuild_StableOrder runs the same expression many times and checks the node order never
// changes -- this is what collecting nodes during PreOrderVisit, instead of from the map behind
// state.IDs(), guarantees.
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
		got := expressions(Build(expr, ast, out, details))
		if want == nil {
			require.NotEmpty(t, got)
			want = got
			continue
		}
		require.Equal(t, want, got, "node order changed on run %d", i)
	}
}
