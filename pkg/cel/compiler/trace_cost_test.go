package compiler

import (
	"context"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apiserver/pkg/cel/environment"
)

// costlyExpression runs 20^5 = 3.2M steps, far over the per-call cost limit that the Kubernetes
// base environment attaches to every program.
const costlyExpression = "[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20].all(a, [1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20].all(b, [1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20].all(c, [1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20].all(d, [1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20].all(e, a+b+c+d+e > 0)))))"

// costLimitedEnv is an env with the Kubernetes per-call cost limit, which NewBaseEnv lacks.
func costLimitedEnv(t *testing.T) *cel.Env {
	t.Helper()
	env, err := environment.MustBaseEnvSet(version.MajorMinor(1, 0)).Env(environment.StoredExpressions)
	require.NoError(t, err)
	return env
}

// TestTracedCompilation_DecidingProgramKeepsCostLimit pins the fix for --explain turning a
// cost-limit error into a pass: state tracking disables cost-limit enforcement in cel-go, so with
// tracing on the program that decides must still be the untracked one, and only the explain-only
// twin may track. Covers every traced compile path: match conditions, variables and validations.
func TestTracedCompilation_DecidingProgramKeepsCostLimit(t *testing.T) {
	env := costLimitedEnv(t)
	type compiled struct{ program, traced cel.Program }
	tests := map[string]func(t *testing.T) compiled{
		"match condition": func(t *testing.T) compiled {
			programs, traced, errs := CompileMatchConditionsWithTrace(field.NewPath("spec"), env, true,
				admissionregistrationv1.MatchCondition{Name: "costly", Expression: costlyExpression})
			require.Empty(t, errs)
			require.Len(t, traced, 1)
			require.Same(t, programs[0], traced[0].Program, "the program returned for deciding must be TracedProgram.Program")
			return compiled{programs[0], traced[0].Traced}
		},
		"variable": func(t *testing.T) compiled {
			programs, traced, errs := CompileVariablesWithTrace(field.NewPath("spec"), env, NewVariablesProvider(env.CELTypeProvider()), true,
				admissionregistrationv1.Variable{Name: "costly", Expression: costlyExpression})
			require.Empty(t, errs)
			return compiled{programs["costly"], traced["costly"].Traced}
		},
		"validation": func(t *testing.T) compiled {
			validation, errs := CompileValidation(field.NewPath("spec"), env, admissionregistrationv1.Validation{Expression: costlyExpression}, true)
			require.Empty(t, errs)
			require.NotNil(t, validation.AST)
			return compiled{validation.Program, validation.Traced}
		},
		"mutation": func(t *testing.T) compiled {
			mutation, errs := CompileMutationWithTrace(field.NewPath("spec"), env, costlyExpression, types.BoolType, true)
			require.Empty(t, errs)
			require.NotNil(t, mutation.AST)
			return compiled{mutation.Program, mutation.Traced}
		},
		"generation": func(t *testing.T) compiled {
			generation, errs := CompileGenerationWithTrace(field.NewPath("spec"), env, policiesv1beta1.Generation{Expression: costlyExpression}, true)
			require.Empty(t, errs)
			require.NotNil(t, generation.AST)
			return compiled{generation.Program, generation.Traced}
		},
	}
	for name, compile := range tests {
		t.Run(name, func(t *testing.T) {
			c := compile(t)
			require.NotNil(t, c.program)
			require.NotNil(t, c.traced)

			_, details, err := c.program.ContextEval(context.Background(), map[string]any{})
			require.Error(t, err, "the deciding program must stop at the cost limit even when compiled for tracing")
			// it does return details, because it tracks cost (that is what enforces the limit),
			// but it must not track per-node values
			if details != nil {
				assert.Nil(t, details.State(), "the deciding program must not track state")
			}

			// documents why the twin must never decide: it runs past the limit
			out, _, err := c.traced.ContextEval(context.Background(), map[string]any{})
			require.NoError(t, err)
			assert.Equal(t, true, out.Value())
		})
	}
}

func TestTraceDetails(t *testing.T) {
	env := costLimitedEnv(t)

	t.Run("re-runs the tracking twin after a normal evaluation", func(t *testing.T) {
		validation, errs := CompileValidation(field.NewPath("spec"), env, admissionregistrationv1.Validation{Expression: "1 + 1 == 2"}, true)
		require.Empty(t, errs)
		_, _, err := validation.Program.ContextEval(context.Background(), map[string]any{})
		require.NoError(t, err)
		details := TraceDetails(context.Background(), validation.Traced, map[string]any{}, err)
		require.NotNil(t, details)
		assert.NotNil(t, details.State())
	})

	t.Run("skips the re-run when the decision hit the cost limit", func(t *testing.T) {
		validation, errs := CompileValidation(field.NewPath("spec"), env, admissionregistrationv1.Validation{Expression: costlyExpression}, true)
		require.Empty(t, errs)
		_, _, err := validation.Program.ContextEval(context.Background(), map[string]any{})
		require.Error(t, err)
		assert.Nil(t, TraceDetails(context.Background(), validation.Traced, map[string]any{}, err),
			"the twin has no cost limit, so it must not be run after a cost-limit cancellation")
	})

	t.Run("is a no-op without a tracking twin", func(t *testing.T) {
		assert.Nil(t, TraceDetails(context.Background(), nil, map[string]any{}, nil))
	})

	t.Run("untraced compilation builds no twin", func(t *testing.T) {
		validation, errs := CompileValidation(field.NewPath("spec"), env, admissionregistrationv1.Validation{Expression: "true"}, false)
		require.Empty(t, errs)
		assert.Nil(t, validation.Traced)
		assert.Nil(t, validation.AST)
	})
}
