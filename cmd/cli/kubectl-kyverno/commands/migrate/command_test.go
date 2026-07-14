package migrate

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandNoResourceFlag(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "at least one resource must be specified"))
}

func TestCommandHelp(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	assert.NoError(t, err)
}
