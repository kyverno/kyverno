package random

import (
	"regexp"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/stretchr/testify/assert"
)

func Test_random(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		Lib(nil),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)

	t.Run("random_string", func(t *testing.T) {
		pattern := "[A-Z0-9]{8}-[A-Z0-9]{4}-[A-Z0-9]{4}-[A-Z0-9]{4}-[A-Z0-9]{12}"
		ast, issues := env.Compile(`random("` + pattern + `")`)
		assert.Nil(t, issues)
		assert.NotNil(t, ast)
		prog, err := env.Program(ast)
		assert.NoError(t, err)
		assert.NotNil(t, prog)
		out, _, err := prog.Eval(map[string]any{})
		assert.NoError(t, err)
		value := out.Value().(string)

		// verify the output matches the pattern
		matched, err := regexp.MatchString("^"+pattern+"$", value)
		assert.NoError(t, err)
		assert.True(t, matched, "generated string %s should match pattern %s", value, pattern)
	})
}

func Test_random_no_param(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		Lib(nil),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)

	t.Run("random_string_no_parameter", func(t *testing.T) {
		defaultPattern := "[0-9a-z]{8}"
		ast, issues := env.Compile(`random()`)
		assert.Nil(t, issues)
		assert.NotNil(t, ast)
		prog, err := env.Program(ast)
		assert.NoError(t, err)
		assert.NotNil(t, prog)
		out, _, err := prog.Eval(map[string]any{})
		assert.NoError(t, err)
		value := out.Value().(string)

		// verify the output matches the pattern
		matched, err := regexp.MatchString("^"+defaultPattern+"$", value)
		assert.NoError(t, err)
		assert.True(t, matched, "generated string %s should match pattern %s", value, defaultPattern)
	})
}
