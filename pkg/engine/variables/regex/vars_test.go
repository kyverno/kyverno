package regex

import (
	"testing"

	"gotest.tools/assert"
)

func Test_RegexVariables(t *testing.T) {
	vars := RegexVariables.FindAllString("tag: {{ value }}", -1)
	assert.Equal(t, len(vars), 1)
	assert.Equal(t, vars[0], " {{ value }}")

	res := RegexVariables.ReplaceAllString("tag: {{ value }}", "${1}test")
	assert.Equal(t, res, "tag: test")
}

func Test_IsVariable(t *testing.T) {
	assert.Equal(t, IsVariable("{{ foo }}"), true)
	assert.Equal(t, IsVariable("{{ foo {{foo2}} }}"), true)
	assert.Equal(t, IsVariable("\\{{ foo }}"), false)
}
