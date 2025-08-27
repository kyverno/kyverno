package completion

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompletionCommand(t *testing.T) {
	cmd := Command()

	// Test command properties
	assert.Equal(t, "completion", cmd.Use[:10])
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotEmpty(t, cmd.Example)
	assert.Equal(t, []string{"bash", "zsh", "fish", "powershell"}, cmd.ValidArgs)
	assert.True(t, cmd.DisableFlagsInUseLine)

	// Test that the command has a valid RunE function
	assert.NotNil(t, cmd.RunE)
}
