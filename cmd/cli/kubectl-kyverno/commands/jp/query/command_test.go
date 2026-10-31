package query

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandInputSource(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		file      string
		content   string
		query     string
		stdin     string
		want      string
		readQuery bool
	}{
		{
			name: "JSON null file", file: "input.json", content: "null", query: "type(@)",
			stdin: `{"unexpected":"stdin"}`, want: "# type(@)\n\"null\"\n",
		},
		{
			name: "YAML null file", file: "input.yaml", content: "~\n", query: "type(@)",
			stdin: "unexpected: stdin", want: "# type(@)\n\"null\"\n",
		},
		{
			name: "null file with query from stdin", file: "input.json", content: "null",
			stdin: "type(@)", want: "# type(@)\n\"null\"\n", readQuery: true,
		},
		{
			name: "object file", file: "input.json", content: `{"source":"file"}`, query: "source",
			stdin: `{"source":"stdin"}`, want: "# source\n\"file\"\n",
		},
		{
			name: "false file", file: "input.yaml", content: "false", query: "type(@)",
			stdin: `{"unexpected":"stdin"}`, want: "# type(@)\n\"boolean\"\n",
		},
		{
			name: "null from stdin", query: "type(@)", stdin: "null",
			want: "# type(@)\n\"null\"\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var args []string
			if tt.file != "" {
				inputPath := filepath.Join(t.TempDir(), tt.file)
				require.NoError(t, os.WriteFile(inputPath, []byte(tt.content), 0o600))
				args = append(args, "--input", inputPath)
			}
			if tt.query != "" {
				args = append(args, tt.query)
			}

			cmd := Command()
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(io.Discard)
			cmd.SetIn(strings.NewReader(tt.stdin))
			cmd.SetArgs(args)
			require.NoError(t, cmd.Execute())
			if tt.readQuery || tt.file == "" {
				assert.True(t, strings.HasSuffix(output.String(), tt.want), output.String())
			} else {
				assert.Equal(t, tt.want, output.String())
			}
		})
	}
}

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
