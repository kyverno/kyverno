package query

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
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

func TestCommandQuery_JSONPipeability(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.json")
	err := os.WriteFile(inputFile, []byte(`{"metadata":{"name":"test-pod"}}`), 0600)
	assert.NoError(t, err)

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "normal JSON query output",
			args: []string{"-i", inputFile, "metadata"},
		},
		{
			name: "compact JSON query output",
			args: []string{"-c", "-i", inputFile, "metadata"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := Command()
			assert.NotNil(t, cmd)
			stdoutBuf := bytes.NewBufferString("")
			stderrBuf := bytes.NewBufferString("")
			cmd.SetOut(stdoutBuf)
			cmd.SetErr(stderrBuf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			assert.NoError(t, err)

			stdoutBytes, err := io.ReadAll(stdoutBuf)
			assert.NoError(t, err)
			stderrBytes, err := io.ReadAll(stderrBuf)
			assert.NoError(t, err)

			var parsed map[string]interface{}
			err = json.Unmarshal(stdoutBytes, &parsed)
			assert.NoError(t, err)
			assert.Equal(t, "test-pod", parsed["name"])

			assert.Contains(t, string(stderrBytes), "# metadata")
			assert.NotContains(t, string(stdoutBytes), "# metadata")
		})
	}
}
