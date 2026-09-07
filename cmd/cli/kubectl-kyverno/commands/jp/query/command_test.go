package query

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommand tests the command execution with valid flags but a missing query file.
func TestCommand(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{"-i", "object.yaml", "-q", "query-file"})
	err := cmd.Execute()
	assert.Error(t, err)
}

// TestCommandWithInvalidArg tests the command execution with missing required arguments.
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

// TestCommandWithInvalidFlag tests the command execution with an unrecognized flag.
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

// TestCommandHelp tests the command execution with the help flag.
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

// TestEvaluateSyntaxError tests the evaluate function for syntax errors and correct caret positioning.
func TestEvaluateSyntaxError(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid",
			query:   "foo.bar",
			wantErr: false,
		},
		{
			name:    "malformed",
			query:   "invalid{{",
			wantErr: true,
			errMsg:  "invalid{{\n        ^",
		},
		{
			name:    "common syntax mistake",
			query:   "foo[",
			wantErr: true,
			errMsg:  "foo[\n    ^",
		},
		{
			name:    "unicode regression",
			query:   "\"é\"[",
			wantErr: true,
			errMsg:  "\"é\"[\n    ^",
		},
		{
			name:    "wide unicode regression",
			query:   "\"界\"[",
			wantErr: true,
			errMsg:  "\"界\"[\n     ^",
		},
		{
			name:    "unquoted multibyte regression",
			query:   "界[",
			wantErr: true,
			errMsg:  "界[\n^",
		},
		{
			name:    "multiline regression",
			query:   "foo.\nbar.\nbaz[",
			wantErr: true,
			errMsg:  "baz[\n    ^",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := evaluate(map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}}, tt.query)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
