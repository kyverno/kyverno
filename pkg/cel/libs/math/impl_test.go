package math

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/stretchr/testify/assert"
)

func Test_round(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		Lib(nil),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)

	t.Run("round", func(t *testing.T) {
		ast, issues := env.Compile(`math.round(10.125, 2)`)
		assert.Nil(t, issues)
		assert.NotNil(t, ast)
		prog, err := env.Program(ast)
		assert.NoError(t, err)
		assert.NotNil(t, prog)
		out, _, err := prog.Eval(map[string]any{})
		assert.NoError(t, err)
		value := out.Value().(float64)
		assert.Equal(t, 10.13, value)
	})

	t.Run("round_zero", func(t *testing.T) {
		ast, issues := env.Compile(`math.round(10.125, 0)`)
		assert.Nil(t, issues)
		assert.NotNil(t, ast)
		prog, err := env.Program(ast)
		assert.NoError(t, err)
		assert.NotNil(t, prog)
		out, _, err := prog.Eval(map[string]any{})
		assert.NoError(t, err)
		value := out.Value().(float64)
		assert.Equal(t, 10.0, value)
	})

	t.Run("round_negative_precision", func(t *testing.T) {
		ast, issues := env.Compile(`math.round(12345.6789, -2)`)
		assert.Nil(t, issues)
		assert.NotNil(t, ast)
		prog, err := env.Program(ast)
		assert.NoError(t, err)
		assert.NotNil(t, prog)
		out, _, err := prog.Eval(map[string]any{})
		assert.NoError(t, err)
		value := out.Value().(float64)
		assert.Equal(t, 12300.0, value)
	})
}
