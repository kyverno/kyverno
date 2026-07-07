package query

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

func TestEvaluateInvalidExpression(t *testing.T) {
	_, err := evaluate(map[string]interface{}{}, "invalid{{")
	assert.Error(t, err)
	// A syntax error should be reported cleanly, quoting the offending
	// expression and highlighting the location (a caret under the bad token)
	// rather than a cryptic message.
	assert.Contains(t, err.Error(), `invalid JMESPath expression "invalid{{"`)
	assert.Contains(t, err.Error(), "SyntaxError:")
	assert.Contains(t, err.Error(), "invalid{{")
	assert.Contains(t, err.Error(), "^")
}

func TestEvaluateValidExpression(t *testing.T) {
	result, err := evaluate(map[string]interface{}{"name": "kyverno"}, "name")
	assert.NoError(t, err)
	assert.Equal(t, "kyverno", result)
}
