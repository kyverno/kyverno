package migrate

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandWithoutResourceFlag(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	assert.Error(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	expected := `Error: required flag(s) "resource" not set`
	assert.True(t, strings.Contains(string(out), expected))
}

func TestCommandWithResourceFlagRunsToConfigLookup(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{"--resource", "foo", "--kubeconfig", "/nonexistent/kubeconfig"})
	err := cmd.Execute()
	assert.Error(t, err)
}
