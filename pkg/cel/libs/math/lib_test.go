package math

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/assert"
)

func Test_MathLib(t *testing.T) {
	v := Latest()
	assert.NotNil(t, v)

	l := Lib(v)
	assert.NotNil(t, l)

	libInstance := &lib{version: v}
	assert.Equal(t, "kyverno.math", libInstance.LibraryName())

	opts := libInstance.CompileOptions()
	assert.NotEmpty(t, opts)

	progOpts := libInstance.ProgramOptions()
	assert.NotNil(t, progOpts)
}

func Test_MathLibRegistration(t *testing.T) {
	env, err := cel.NewEnv(Lib(Latest()))
	assert.NoError(t, err)
	assert.NotNil(t, env)
}
