package scan

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommand(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	err := cmd.Execute()
	assert.Error(t, err)
}

func TestCommandInvalidFileName(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{"foo", "-f", ""})
	err := cmd.Execute()
	assert.Error(t, err)
}

// func TestCommandWithInvalidArg(t *testing.T) {
// 	cmd := Command()
// 	assert.NotNil(t, cmd)
// 	b := bytes.NewBufferString("")
// 	cmd.SetErr(b)
// 	err := cmd.Execute()
// 	assert.Error(t, err)
// 	out, err := io.ReadAll(b)
// 	assert.NoError(t, err)
// 	expected := `Error: requires at least 1 arg(s), only received 0`
// 	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(out)))
// }

func TestCommandWithInvalidFlag(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	cmd.SetArgs([]string{"--xxx"})
	err := cmd.Execute()
	assert.Error(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	expected := `Error: unknown flag: --xxx`
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(out)))
}

func TestCommandHelp(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	assert.NoError(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(out), cmd.Long))
}
