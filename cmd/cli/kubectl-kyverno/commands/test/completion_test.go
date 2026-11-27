package test

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputFormatCompletion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "empty", args: []string{"--output-format="}, want: "json\nyaml\nmarkdown\njunit\n:4\n"},
		{name: "prefix", args: []string{"--output-format=ju"}, want: "junit\n:4\n"},
		{name: "shared prefix", args: []string{"--output-format=j"}, want: "json\njunit\n:4\n"},
		{name: "exact", args: []string{"--output-format=junit"}, want: "junit\n:4\n"},
		{name: "unknown", args: []string{"--output-format=xml"}, want: ":4\n"},
		{name: "case mismatch", args: []string{"--output-format=JUNIT"}, want: ":4\n"},
		{name: "separate value", args: []string{"--output-format", "m"}, want: "markdown\n:4\n"},
		{name: "shorthand empty", args: []string{"-o", ""}, want: "json\nyaml\nmarkdown\njunit\n:4\n"},
		{name: "shorthand prefix", args: []string{"-o", "ju"}, want: "junit\n:4\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := &cobra.Command{Use: "kyverno"}
			cmd := Command()
			cmd.RunE = func(_ *cobra.Command, _ []string) error {
				t.Fatal("completion must not execute policy tests")
				return nil
			}
			root.AddCommand(cmd)
			var out, errOut bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&errOut)
			root.SetArgs(append([]string{"__complete", "test", "missing-test-directory"}, tt.args...))

			require.NoError(t, root.Execute())
			assert.Equal(t, tt.want, out.String())
		})
	}
}
