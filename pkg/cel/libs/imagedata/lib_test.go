package imagedata

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/stretchr/testify/assert"
)

func TestLib(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("image", ContextType),
		Lib(),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
}

func Test_lib_LibraryName(t *testing.T) {
	var l lib
	assert.Equal(t, libraryName, l.LibraryName())
}
