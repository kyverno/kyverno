package commands

import (
	"bytes"
	"io"
	"strings"
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

func TestRootCommandWithInvalidArg(t *testing.T) {
	cmd := RootCommand(false)
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	cmd.SetArgs([]string{"foo"})
	err := cmd.Execute()
	assert.Error(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	expected := `
Error: unknown command "foo" for "kyverno"
Run 'kyverno --help' for usage.`
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(out)))
}

func TestRootCommandWithInvalidFlag(t *testing.T) {
	cmd := RootCommand(false)
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

func TestRootCommandHelp(t *testing.T) {
	cmd := RootCommand(false)
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
