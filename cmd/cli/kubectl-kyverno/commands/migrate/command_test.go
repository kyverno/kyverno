package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandWithoutResourceFlag(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	assert.ErrorContains(t, err, `required flag(s) "resource" not set`)
}

func TestCommandWithResourceFlagRunsToConfigLookup(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{"--resource", "foo", "--kubeconfig", "/nonexistent/kubeconfig"})
	err := cmd.Execute()
	assert.Error(t, err)
}
