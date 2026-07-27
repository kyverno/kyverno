package compiler

import (
	"context"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/stretchr/testify/assert"
)

// mockVpolProgram is a lightweight cel.Program stub for unit tests.
type mockVpolProgram struct {
	retVal ref.Val
	err    error
}

func (m *mockVpolProgram) ContextEval(_ context.Context, _ any) (ref.Val, *cel.EvalDetails, error) {
	return m.retVal, nil, m.err
}

func (m *mockVpolProgram) Eval(any) (ref.Val, *cel.EvalDetails, error) {
	return m.retVal, nil, m.err
}

// TestEvaluateWithData_FullExemptionPrecedence is a regression test for
// https://github.com/kyverno/kyverno/issues/16053.
//
// When multiple PolicyExceptions match a resource, a full-exemption exception
// (one with no Images and no AllowedValues) must cause the evaluation loop to
// break and indicate a full exemption, regardless of whether a partial
// exception also matched.
func TestEvaluateWithData_FullExemptionPrecedence(t *testing.T) {
	t.Run("full-exemption takes precedence over partial exception", func(t *testing.T) {
		partialEx := &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				Images: []string{"nginx:*"},
			},
		}
		fullEx := &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				// no Images, no AllowedValues → full exemption
			},
		}

		p := &Policy{
			exceptions: []compiler.Exception{
				// partial exception is matched first
				{MatchConditions: []cel.Program{}, Exception: partialEx},
				// full exemption is matched second – must still win
				{MatchConditions: []cel.Program{}, Exception: fullEx},
			},
		}

		result, err := p.evaluateWithData(context.Background(), evaluationData{})

		assert.NoError(t, err)
		// The full exemption breaks the loop (resetting any prior partial scopes),
		// and the post-loop check returns with exceptions; no validation is run.
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.Exceptions)
		assert.False(t, result.Result, "validation should not have run")
	})

	t.Run("full-exemption takes precedence when appearing first", func(t *testing.T) {
		partialEx := &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				Images: []string{"nginx:*"},
			},
		}
		fullEx := &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				// no Images, no AllowedValues → full exemption
			},
		}

		p := &Policy{
			exceptions: []compiler.Exception{
				// full exemption is matched first
				{MatchConditions: []cel.Program{}, Exception: fullEx},
				// partial exception is matched second
				{MatchConditions: []cel.Program{}, Exception: partialEx},
			},
		}

		result, err := p.evaluateWithData(context.Background(), evaluationData{})

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.Exceptions)
		assert.False(t, result.Result, "validation should not have run")
	})

	t.Run("partial exception alone does not skip evaluation", func(t *testing.T) {
		partialEx := &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				Images: []string{"nginx:*"},
			},
		}

		// A single validation that always returns true.
		alwaysPass := &mockVpolProgram{retVal: types.Bool(true)}

		p := &Policy{
			exceptions: []compiler.Exception{
				{MatchConditions: []cel.Program{}, Exception: partialEx},
			},
			validations: []compiler.Validation{
				{Program: alwaysPass},
			},
		}

		result, err := p.evaluateWithData(context.Background(), evaluationData{})

		// A partial exception alone must NOT skip validation; the policy is evaluated.
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Result, "validation should have run and passed")
	})

	t.Run("all matched exceptions collected when priority labels and reportResult differ", func(t *testing.T) {
		// Regression guard for the exhaustive-loop requirement from the maintainer review:
		// matchedExceptions must be complete so the engine can (a) pick the
		// highest-priority exception via polex.kyverno.io/priority and (b) build
		// the user-facing message that lists every matched exception key.
		//
		// highPriorityEx has priority=10 (the winner for report selection).
		// laterEx has a lower priority but carries reportResult: pass, which
		// would silently override the skip result if the engine only saw *it*.
		// With the old break-based loop the second exception was never collected;
		// with the flag-based loop both must appear in result.Exceptions.
		highPriorityEx := &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				// full exemption – no Images, no AllowedValues
			},
		}
		highPriorityEx.SetLabels(map[string]string{
			"polex.kyverno.io/priority": "10",
		})

		laterEx := &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				// full exemption as well; carries reportResult: pass
				ReportResult: "pass",
			},
		}
		laterEx.SetLabels(map[string]string{
			"polex.kyverno.io/priority": "5",
		})

		p := &Policy{
			exceptions: []compiler.Exception{
				// high-priority exception is first in iteration order
				{MatchConditions: []cel.Program{}, Exception: highPriorityEx},
				// lower-priority exception with reportResult: pass comes second
				{MatchConditions: []cel.Program{}, Exception: laterEx},
			},
		}

		result, err := p.evaluateWithData(context.Background(), evaluationData{})

		assert.NoError(t, err)
		assert.NotNil(t, result)
		// Both exceptions must be present so the engine sees the complete set.
		assert.Len(t, result.Exceptions, 2, "both exceptions must be collected by the exhaustive loop")
	})
}
