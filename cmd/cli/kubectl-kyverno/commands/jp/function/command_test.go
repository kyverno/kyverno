package function

import (
	"bytes"
	"io"
	"strings"
	"testing"

	fatihcolor "github.com/fatih/color"
	"github.com/stretchr/testify/assert"
)

func TestCommand(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	err := cmd.Execute()
	assert.NoError(t, err)
}

func TestCommandWithOneArg(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{"truncate"})
	err := cmd.Execute()
	assert.NoError(t, err)
}

func TestCommandWithArgs(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{"truncate", "to_upper"})
	err := cmd.Execute()
	assert.NoError(t, err)
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

func TestCommandWithRemoveColor(t *testing.T) {
	origNoColor := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = origNoColor
	})
	fatihcolor.NoColor = false

	// Without --remove-color, color output should be enabled and contain ANSI escape sequences
	cmdWithColor := Command()
	assert.NotNil(t, cmdWithColor)
	bWithColor := bytes.NewBufferString("")
	cmdWithColor.SetOut(bWithColor)
	cmdWithColor.SetArgs([]string{"to_upper"})
	err := cmdWithColor.Execute()
	assert.NoError(t, err)
	outWithColor, err := io.ReadAll(bWithColor)
	assert.NoError(t, err)
	assert.Contains(t, string(outWithColor), "\x1b[36;1mto_upper\x1b[0;22m")
	assert.Contains(t, string(outWithColor), "\x1b[33;1mtakes in a string and outputs the same string with all upper-case letters\x1b[0;22m")

	// With --remove-color, output should be plain text without any ANSI escape sequences
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetArgs([]string{"--remove-color", "to_upper"})
	err = cmd.Execute()
	assert.NoError(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	assert.NotContains(t, string(out), "\x1b[")
	expected := "Name: to_upper\n  Signature: to_upper(string) string\n  Note:      takes in a string and outputs the same string with all upper-case letters"
	assert.Contains(t, strings.TrimSpace(string(out)), expected)
}
