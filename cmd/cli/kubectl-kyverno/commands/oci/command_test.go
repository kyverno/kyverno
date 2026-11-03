package oci

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
	assert.NoError(t, err)
}

func TestCommandWithArgs(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{"foo"})
	err := cmd.Execute()
	assert.Error(t, err)
}

func TestCommandWithInvalidArg(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	cmd.SetArgs([]string{"foo"})
	err := cmd.Execute()
	assert.Error(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	expected := `Error: unknown command "foo" for "oci"`
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(out)))
}

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

<<<<<<< HEAD
=======
// TestCommandRegistersValidateSubcommand pins that `kyverno oci validate` is wired in alongside
// push and pull, not just importable from its own package.
func TestCommandRegistersValidateSubcommand(t *testing.T) {
	cmd := Command()
	sub, _, err := cmd.Find([]string{"validate"})
	require.NoError(t, err)
	assert.Equal(t, "validate", sub.Name())
}

// TestCommandRegistersInspectSubcommand pins that `kyverno oci inspect` is wired in, not just
// importable from its own package.
func TestCommandRegistersInspectSubcommand(t *testing.T) {
	cmd := Command()
	sub, _, err := cmd.Find([]string{"inspect"})
	require.NoError(t, err)
	assert.Equal(t, "inspect", sub.Name())
}

// TestCommandRegistersResolveSubcommand pins that `kyverno oci resolve` is wired in, not just
// importable from its own package.
func TestCommandRegistersResolveSubcommand(t *testing.T) {
	cmd := Command()
	sub, _, err := cmd.Find([]string{"resolve"})
	require.NoError(t, err)
	assert.Equal(t, "resolve", sub.Name())
}

>>>>>>> 9a39e3a58 (feat(cli): add kyverno oci inspect and oci resolve (#18420))
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
