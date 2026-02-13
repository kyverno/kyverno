package time

import (
	"testing"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/stretchr/testify/assert"
)

func Test_time_now(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		Lib(nil),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)

	t.Run("time_now", func(t *testing.T) {
		ast, issues := env.Compile(`time.now() - duration("3h")`)
		assert.Nil(t, issues)
		assert.NotNil(t, ast)
		prog, err := env.Program(ast)
		assert.NoError(t, err)
		assert.NotNil(t, prog)
		out, _, err := prog.Eval(map[string]any{})
		_ = out.Value().(time.Time) // assert that the output is a timestamp
		assert.NoError(t, err)
	})
}

func Test_time_truncate(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		Lib(nil),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)

	t.Run("time_truncate", func(t *testing.T) {
		expr := `
			time.truncate(
				timestamp("2025-01-02T03:45:27Z"),
				duration("1h")
			)
		`
		ast, issues := env.Compile(expr)
		assert.Nil(t, issues)
		assert.NotNil(t, ast)

		prog, err := env.Program(ast)
		assert.NoError(t, err)
		assert.NotNil(t, prog)

		out, _, err := prog.Eval(map[string]any{})
		assert.NoError(t, err)
		assert.NotNil(t, out)

		// validate the truncated timestamp
		got := out.Value().(time.Time)
		expected := time.Date(2025, 1, 2, 3, 0, 0, 0, time.UTC)
		assert.Equal(t, expected, got)
	})
}

func Test_time_toCron(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		Lib(nil),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)

	t.Run("time_toCron", func(t *testing.T) {
		expr := `time.toCron(timestamp("2025-01-02T15:30:00Z"))`
		ast, issues := env.Compile(expr)
		assert.Nil(t, issues)
		assert.NotNil(t, ast)

		prog, err := env.Program(ast)
		assert.NoError(t, err)
		assert.NotNil(t, prog)

		out, _, err := prog.Eval(map[string]any{})
		assert.NoError(t, err)
		assert.NotNil(t, out)

		got := out.Value().(string)
		expected := "30 15 2 1 4"
		assert.Equal(t, expected, got)
	})
}
