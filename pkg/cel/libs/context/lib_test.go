package context

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/assert"
)

func TestLib(t *testing.T) {
	lib := Lib()
	env, err := cel.NewEnv(cel.Lib(lib))
	assert.NoError(t, err)
	assert.NotNil(t, env)
}

func Test_lib_LibraryName(t *testing.T) {
	var l lib
	assert.Equal(t, libraryName, l.LibraryName())
}
