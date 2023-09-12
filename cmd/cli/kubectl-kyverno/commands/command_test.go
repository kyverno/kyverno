package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootCommand(t *testing.T) {
	cmd := RootCommand(false)
	assert.NotNil(t, cmd)
	assert.Len(t, cmd.Commands(), 6)
	err := cmd.Execute()
	assert.NoError(t, err)
}

func TestRootCommandExperimental(t *testing.T) {
	cmd := RootCommand(true)
	assert.NotNil(t, cmd)
	assert.Len(t, cmd.Commands(), 8)
	err := cmd.Execute()
	assert.NoError(t, err)
}
