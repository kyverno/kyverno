package compiler

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/kyverno/sdk/extensions/cel/libs/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// countingHTTP is an http library implementation that counts the requests made through it and
// never reaches the network.
type countingHTTP struct{ calls *atomic.Int32 }

func (c countingHTTP) Get(string, map[string]string) (any, error) {
	c.calls.Add(1)
	return map[string]any{"approved": false}, nil
}

func (c countingHTTP) Post(string, any, map[string]string) (any, error) {
	c.calls.Add(1)
	return map[string]any{"approved": false}, nil
}

func (c countingHTTP) Client(string) (http.ContextInterface, error) { return c, nil }

func countingHTTPEnv(t *testing.T) (*cel.Env, *atomic.Int32) {
	t.Helper()
	calls := &atomic.Int32{}
	env, err := costLimitedEnv(t).Extend(http.Lib(http.Context{ContextInterface: countingHTTP{calls: calls}}, http.Latest()))
	require.NoError(t, err)
	return env, calls
}

// TestTracedCompilation_ExternalCallRunsOnce: with tracing on, a policy evaluates an expression
// with its normal program and then re-runs the tracking twin to explain it. For an expression that
// calls http.Post that would send the request twice, so it gets no twin: deciding and explaining
// it together make exactly one request. Every policy type builds its twins here, so this holds for
// all of them.
func TestTracedCompilation_ExternalCallRunsOnce(t *testing.T) {
	const expression = `http.Post('https://approvals.example', {'pod': 'web'}).approved == true`
	type compiled struct{ program, traced cel.Program }
	tests := map[string]func(t *testing.T, env *cel.Env) compiled{
		"validation": func(t *testing.T, env *cel.Env) compiled {
			v, errs := CompileValidation(field.NewPath("spec"), env, admissionregistrationv1.Validation{Expression: expression}, true)
			require.Empty(t, errs)
			require.NotNil(t, v.AST, "the AST is kept, so the trace still shows the expression")
			return compiled{v.Program, v.Traced}
		},
		"match condition": func(t *testing.T, env *cel.Env) compiled {
			programs, traced, errs := CompileMatchConditionsWithTrace(field.NewPath("spec"), env, true, admissionregistrationv1.MatchCondition{Name: "approved", Expression: expression})
			require.Empty(t, errs)
			return compiled{programs[0], traced[0].Traced}
		},
		"mutation": func(t *testing.T, env *cel.Env) compiled {
			m, errs := CompileMutationWithTrace(field.NewPath("spec"), env, expression, types.BoolType, true)
			require.Empty(t, errs)
			return compiled{m.Program, m.Traced}
		},
	}
	for name, compile := range tests {
		t.Run(name, func(t *testing.T) {
			env, calls := countingHTTPEnv(t)
			c := compile(t, env)
			require.NotNil(t, c.program)
			assert.Nil(t, c.traced, "an expression that calls http.Post gets no tracking twin")

			// decide, then explain, as a traced policy evaluation does
			out, _, err := c.program.ContextEval(context.Background(), map[string]any{})
			require.NoError(t, err)
			assert.Equal(t, false, out.Value())
			assert.Nil(t, TraceDetails(context.Background(), c.traced, map[string]any{}, err), "nothing to re-run")
			assert.Equal(t, int32(1), calls.Load(), "the request is sent once")
		})
	}

	// why the twin must not exist: re-running the expression with a tracking program sends the
	// request again
	t.Run("a tracking program would send it again", func(t *testing.T) {
		env, calls := countingHTTPEnv(t)
		ast, iss := env.Compile(expression)
		require.NoError(t, iss.Err())
		twin, err := env.Program(ast, cel.EvalOptions(cel.OptTrackState))
		require.NoError(t, err)
		_, _, err = twin.ContextEval(context.Background(), map[string]any{})
		require.NoError(t, err)
		_, _, err = twin.ContextEval(context.Background(), map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, int32(2), calls.Load())
	})
}

func TestNotRepeatable(t *testing.T) {
	env, _ := countingHTTPEnv(t)
	compile := func(expression string) *cel.Ast {
		ast, iss := env.Compile(expression)
		require.NoError(t, iss.Err())
		return ast
	}
	assert.Equal(t, "http.Get", NotRepeatable(compile(`http.Get('https://a.example').approved == true`)))
	assert.Equal(t, "http.Post", NotRepeatable(compile(`http.Client('').Post('https://a.example', {}).approved == true`)), "a call on a client built from http is caught too")
	assert.Equal(t, "", NotRepeatable(compile(`1 + 1 == 2`)))
	assert.Equal(t, "", NotRepeatable(nil))
	assert.Equal(t, "it calls http.Get, which is not run a second time", NoBreakdownReason(compile(`http.Get('https://a.example').approved == true`)))
	assert.Equal(t, "", NoBreakdownReason(compile(`1 + 1 == 2`)))

	// an expression that only reads its inputs keeps its twin
	v, errs := CompileValidation(field.NewPath("spec"), env, admissionregistrationv1.Validation{Expression: `1 + 1 == 2`}, true)
	require.Empty(t, errs)
	assert.NotNil(t, v.Traced)
}
