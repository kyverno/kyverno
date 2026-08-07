package query

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommand(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{"-i", "object.yaml", "-q", "query-file"})
	err := cmd.Execute()
	assert.Error(t, err)
}

func TestCommandWithInvalidArg(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	err := cmd.Execute()
	assert.Error(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	expected := `Error: at least one query or input object is required`
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

func TestEvaluateInvalidSyntax(t *testing.T) {
	// An invalid JMESPath expression should return an error containing "syntax"
	_, err := evaluate(map[string]interface{}{"foo": "bar"}, "invalid[expression")
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "syntax")
}

func TestEvaluateValidExpression(t *testing.T) {
	// A valid JMESPath expression with valid input should succeed
	input := map[string]interface{}{"foo": "bar"}
	result, err := evaluate(input, "foo")
	require.NoError(t, err)
	assert.Equal(t, "bar", result)
}
