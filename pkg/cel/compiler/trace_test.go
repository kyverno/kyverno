package compiler

import (
	"context"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/assert"
)

// Real compile + eval with OptTrackState, not a mock.
func TestCaptureTrace_RealEvaluation(t *testing.T) {
	env, err := cel.NewEnv(cel.Variable("ns", cel.StringType))
	assert.NoError(t, err)

	ast, issues := env.Compile(`ns == "prod"`)
	assert.NoError(t, issues.Err())

	prog, err := env.Program(ast, cel.EvalOptions(cel.OptTrackState))
	assert.NoError(t, err)

	_, details, err := prog.ContextEval(context.Background(), map[string]any{
		"ns": "default",
	})
	assert.NoError(t, err)
	assert.NotNil(t, details)

	entries := CaptureTrace("match", "only-prod", ast, details)
	assert.NotEmpty(t, entries)

	found := false
	for _, e := range entries {
		if e.Value == "false" {
			found = true
		}
		assert.Equal(t, "match", e.Layer)
		assert.Equal(t, "only-prod", e.Name)
		assert.True(t, e.Evaluated)
	}
	assert.True(t, found)
}

func TestCaptureTrace_NilDetails(t *testing.T) {
	env, err := cel.NewEnv(cel.Variable("ns", cel.StringType))
	assert.NoError(t, err)

	ast, issues := env.Compile(`ns == "prod"`)
	assert.NoError(t, issues.Err())

	prog, err := env.Program(ast)
	assert.NoError(t, err)

	_, details, err := prog.ContextEval(context.Background(), map[string]any{
		"ns": "default",
	})
	assert.NoError(t, err)
	assert.Nil(t, details)

	entries := CaptureTrace("match", "only-prod", ast, details)
	assert.Nil(t, entries)
}

func TestNotEvaluated(t *testing.T) {
	entry := NotEvaluated("match", "only-staging", `ns == "staging"`)
	assert.Equal(t, "match", entry.Layer)
	assert.Equal(t, "only-staging", entry.Name)
	assert.False(t, entry.Evaluated)
	assert.Empty(t, entry.Value)
}
