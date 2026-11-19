package version

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputCompletion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "empty", args: []string{"--output="}, want: "json\nyaml\n:4\n"},
		{name: "prefix", args: []string{"--output=j"}, want: "json\n:4\n"},
		{name: "exact", args: []string{"--output=yaml"}, want: "yaml\n:4\n"},
		{name: "unknown", args: []string{"--output=xml"}, want: ":4\n"},
		{name: "case mismatch", args: []string{"--output=JSON"}, want: ":4\n"},
		{name: "separate value", args: []string{"--output", "y"}, want: "yaml\n:4\n"},
		{name: "shorthand empty", args: []string{"-o", ""}, want: "json\nyaml\n:4\n"},
		{name: "shorthand prefix", args: []string{"-o", "j"}, want: "json\n:4\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := &cobra.Command{Use: "kyverno"}
			cmd := Command()
			cmd.RunE = func(_ *cobra.Command, _ []string) error {
				t.Fatal("completion must not execute the version command")
				return nil
			}
			root.AddCommand(cmd)
			var out, errOut bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&errOut)
			root.SetArgs(append([]string{"__complete", "version"}, tt.args...))

			require.NoError(t, root.Execute())
			assert.Equal(t, tt.want, out.String())
		})
	}
}
