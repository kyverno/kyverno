package jp

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
	expected := `Error: unknown command "foo" for "jp"`
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

// TestCommandHelpStaysAvailable guards against reaching for cobra's Deprecated field instead
// of the custom notice: Deprecated hides a command from its parent's help listing, which
// would bury the very command users need to debug JMESPath expressions during migration.
func TestCommandHelpStaysAvailable(t *testing.T) {
	cmd := Command()
	assert.Empty(t, cmd.Deprecated)
	assert.True(t, cmd.IsAvailableCommand())
	for _, sub := range cmd.Commands() {
		assert.Empty(t, sub.Deprecated, sub.Name())
		assert.True(t, sub.IsAvailableCommand(), sub.Name())
	}
}

// TestCommandPrintsDeprecationNoticeToStderr ensures the notice lands on stderr, not stdout,
// so it never corrupts the output of a piped `jp query`/`jp parse` invocation.
func TestCommandPrintsDeprecationNoticeToStderr(t *testing.T) {
	cmd := Command()
	outBuf := bytes.NewBufferString("")
	errBuf := bytes.NewBufferString("")
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"function"})
	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, errBuf.String(), deprecationNotice)
	assert.NotContains(t, outBuf.String(), deprecationNotice)
}
