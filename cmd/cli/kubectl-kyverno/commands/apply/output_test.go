package apply

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

func TestResolveOutputFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		table   bool
		warning string
		err     string
	}{
		{name: "default"},
		{name: "table format", args: []string{"--output-format", "table"}, table: true},
		{name: "legacy table", args: []string{"-t"}, table: true},
		{name: "json report", args: []string{"-p", "--output-format", "json"}},
		{name: "yaml exceptions", args: []string{"--generate-exceptions", "--output-format", "yaml"}},
		{name: "ignored json", args: []string{"--output-format", "json"}, warning: "has no effect without --policy-report"},
		{name: "explicit default yaml", args: []string{"--output-format", "yaml"}, warning: "has no effect without --policy-report"},
		{name: "report wins over table", args: []string{"-p", "-t"}, table: true, warning: "--table is ignored"},
		{name: "exceptions win over table", args: []string{"--generate-exceptions", "-t"}, table: true, warning: "--table is ignored"},
		{name: "report wins over exceptions", args: []string{"-p", "--generate-exceptions"}, warning: "--generate-exceptions is ignored"},
		{name: "details without table", args: []string{"--detailed-results"}, warning: "--detailed-results has no effect"},
		{name: "table details", args: []string{"--output-format", "table", "--detailed-results"}, table: true},
		{name: "table report", args: []string{"-p", "--output-format", "table"}, err: "cannot be used with"},
		{name: "table exceptions", args: []string{"--generate-exceptions", "--output-format", "table"}, err: "cannot be used with"},
		{name: "unknown format without reports", args: []string{"--output-format", "jsno"}, warning: "--output-format jsno has no effect"},
		{name: "unknown report format falls back to yaml", args: []string{"-p", "--output-format", "jsno"}, warning: "unsupported output format \"jsno\"; using yaml"},
		{name: "unknown exception format falls back to yaml", args: []string{"--generate-exceptions", "--output-format", "jsno"}, warning: "unsupported output format \"jsno\"; using yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := Command()
			var out, warnings bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&warnings)
			require.NoError(t, cmd.ParseFlags(tt.args))
			// Discard pflag's deprecation messages to isolate mode warnings.
			warnings.Reset()
			out.Reset()
			config := &ApplyCommandConfig{}
			config.OutputFormat, _ = cmd.Flags().GetString("output-format")
			config.PolicyReport, _ = cmd.Flags().GetBool("policy-report")
			config.GenerateExceptions, _ = cmd.Flags().GetBool("generate-exceptions")
			table, _ := cmd.Flags().GetBool("table")
			details, _ := cmd.Flags().GetBool("detailed-results")
			got, err := config.resolveOutputFlags(cmd, table, details)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.table, got)
			if tt.warning == "" {
				assert.Empty(t, warnings.String())
			} else {
				assert.Contains(t, warnings.String(), tt.warning)
			}
			assert.Empty(t, out.String(), "warnings must not pollute stdout")
		})
	}
}

func TestLegacyOutputRejectsFormats(t *testing.T) {
	fixtures, err := filepath.Abs("../../../../../test/conformance/chainsaw/cli/apply/output-flags/")
	require.NoError(t, err)
	for _, flag := range []string{"-o", "--output"} {
		for _, format := range []string{"json", "yaml", "yml", "table", "markdown", "junit"} {
			for _, path := range []string{format, strings.ToUpper(format), "./" + format} {
				t.Run(flag+"/"+path, func(t *testing.T) {
					dir := t.TempDir()
					t.Chdir(dir)
					cmd := Command()
					cmd.SetOut(io.Discard)
					cmd.SetErr(io.Discard)
					cmd.SetArgs([]string{filepath.Join(fixtures, "policy.yaml"), "-r", filepath.Join(fixtures, "resource.yaml"), flag, path})
					err := cmd.Execute()
					require.ErrorContains(t, err, "invalid path")
					assert.ErrorContains(t, err, "--output-format")
					assert.NotContains(t, err.Error(), "--output-format "+format)
					entries, err := os.ReadDir(dir)
					require.NoError(t, err)
					assert.Empty(t, entries, "rejected formats must not create files or directories")
				})
			}
		}
	}
}

func TestExplicitOutputAcceptsFormatNames(t *testing.T) {
	fixtures, err := filepath.Abs("../../../../../test/conformance/chainsaw/cli/apply/output-flags/")
	require.NoError(t, err)
	for _, flag := range []string{"--output-dir", "--output-file"} {
		for _, name := range []string{"json", "yaml", "yml", "table", "markdown", "junit", "JSON"} {
			t.Run(flag+"/"+name, func(t *testing.T) {
				t.Chdir(t.TempDir())
				cmd := Command()
				cmd.SetOut(io.Discard)
				cmd.SetErr(io.Discard)
				cmd.SetArgs([]string{filepath.Join(fixtures, "policy.yaml"), "-r", filepath.Join(fixtures, "resource.yaml"), flag, "./" + name})
				require.NoError(t, cmd.Execute())
				info, err := os.Stat(name)
				require.NoError(t, err)
				assert.Equal(t, flag == "--output-dir", info.IsDir())
				path := name
				if info.IsDir() {
					path = filepath.Join(name, "output-example-mutated.yaml")
				}
				data, err := os.ReadFile(path)
				require.NoError(t, err)
				assert.Contains(t, string(data), "output-test: \"true\"")
			})
		}
	}
}

func TestOutputDestinationTypes(t *testing.T) {
	for _, directory := range []bool{false, true} {
		name := "file without extension"
		if directory {
			name = "directory with yaml extension"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "results")
			config := &ApplyCommandConfig{OutputFile: path}
			if directory {
				path += ".yaml"
				config = &ApplyCommandConfig{OutputDir: path}
			}
			got, err := config.getMutateLogPathIsDir()
			require.NoError(t, err)
			assert.Equal(t, directory, got)
			assert.Equal(t, path, config.MutateLogPath)
			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, directory, info.IsDir())
			opposite := &ApplyCommandConfig{OutputDir: path}
			if directory {
				opposite = &ApplyCommandConfig{OutputFile: path}
			}
			_, err = opposite.getMutateLogPathIsDir()
			require.ErrorContains(t, err, "wrong type")
		})
	}
}

func TestOutputDestinationsMutuallyExclusive(t *testing.T) {
	for _, args := range [][]string{
		{"--output", "old", "--output-dir", "new"},
		{"--output", "old", "--output-file", "new"},
		{"--output-dir", "dir", "--output-file", "file"},
	} {
		t.Run(args[0]+args[2], func(t *testing.T) {
			cmd := Command()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(args)
			require.ErrorContains(t, cmd.Execute(), "none of the others can be")
		})
	}
}

func TestInvalidInputPreservesOutput(t *testing.T) {
	const fixtures = "../../../../../test/conformance/chainsaw/cli/apply/output-flags/"
	tests := []struct {
		name          string
		missingValues bool
		existing      bool
	}{
		{name: "missing values file, new destination", missingValues: true},
		{name: "missing values file, existing destination", missingValues: true, existing: true},
		{name: "missing resource, new destination"},
		{name: "missing resource, existing destination", existing: true},
	}
	for _, flag := range []string{"--output", "--output-dir", "--output-file"} {
		for _, tt := range tests {
			t.Run(flag+"/"+tt.name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "output.yaml")
				original := path
				if tt.existing {
					if flag == "--output-dir" {
						require.NoError(t, os.Mkdir(path, 0o750))
						original = filepath.Join(path, "keep.yaml")
					}
					require.NoError(t, os.WriteFile(original, []byte("keep me"), 0o600))
				}
				policy := fixtures + "policy.yaml"
				resource := filepath.Join(t.TempDir(), "missing-resource.yaml")
				args := []string{policy, "-r", resource, flag, path}
				if tt.missingValues {
					args = []string{policy, "-r", fixtures + "resource.yaml", flag, path,
						"--values-file", filepath.Join(t.TempDir(), "missing-values.yaml")}
				}
				cmd := Command()
				cmd.SetOut(io.Discard)
				cmd.SetErr(io.Discard)
				cmd.SetArgs(args)
				require.Error(t, cmd.Execute())
				if tt.existing {
					content, err := os.ReadFile(original)
					require.NoError(t, err)
					assert.Equal(t, "keep me", string(content))
				} else {
					_, err := os.Stat(path)
					require.True(t, os.IsNotExist(err))
				}
			})
		}
	}
}

func TestInvalidOutputPreservesDestination(t *testing.T) {
	const fixtures = "../../../../../test/conformance/chainsaw/cli/apply/output-flags/"
	for _, tt := range []struct {
		name  string
		flags []string
		err   string
	}{
		{name: "table report", flags: []string{"--policy-report", "--output-format", "table"}, err: "--output-format table cannot be used"},
		{name: "table exceptions", flags: []string{"--generate-exceptions", "--output-format", "table"}, err: "--output-format table cannot be used"},
		{name: "legacy file path is a directory", err: "failed to truncate the existing file"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "output.yaml")
			require.NoError(t, os.Mkdir(destination, 0o750))
			original := filepath.Join(destination, "keep.yaml")
			require.NoError(t, os.WriteFile(original, []byte("keep me"), 0o600))
			cmd := Command()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			args := []string{fixtures + "policy.yaml", "-r", fixtures + "resource.yaml", "--output", destination}
			cmd.SetArgs(append(args, tt.flags...))
			require.ErrorContains(t, cmd.Execute(), tt.err)
			content, err := os.ReadFile(original)
			require.NoError(t, err)
			assert.Equal(t, "keep me", string(content))
			entries, err := os.ReadDir(destination)
			require.NoError(t, err)
			require.Len(t, entries, 1)
			assert.Equal(t, "keep.yaml", entries[0].Name())
		})
	}
}

func TestCommandOutputFlags(t *testing.T) {
	const fixtures = "../../../../../test/conformance/chainsaw/cli/apply/output-flags/"
	run := func(t *testing.T, flags ...string) (string, string) {
		t.Helper()
		cmd := Command()
		var out, warnings bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&warnings)
		cmd.SetArgs(append([]string{fixtures + "policy.yaml", "-r", fixtures + "resource.yaml", "--remove-color"}, flags...))
		require.NoError(t, cmd.Execute())
		return out.String(), warnings.String()
	}
	t.Run("mutation counted once", func(t *testing.T) {
		out, warnings := run(t)
		assert.Empty(t, warnings)
		assert.Contains(t, out, "pass: 1, fail: 0, warn: 0, error: 0, skip: 0")
	})
	t.Run("unknown formats preserve behavior", func(t *testing.T) {
		out, warnings := run(t, "--output-format", "jsno")
		assert.Contains(t, out, "pass: 1")
		assert.Contains(t, warnings, "--output-format jsno has no effect")
		out, warnings = run(t, "-p", "--output-format", "jsno")
		assert.Contains(t, out, "kind: ClusterReport")
		assert.Contains(t, warnings, "using yaml")
	})
	t.Run("table matches legacy", func(t *testing.T) {
		table, warnings := run(t, "--output-format", "table")
		assert.Contains(t, table, "add-output-label")
		assert.Contains(t, table, "Pass")
		assert.Empty(t, warnings)
		for _, flag := range []string{"-t", "--table"} {
			t.Run(flag, func(t *testing.T) {
				legacy, legacyWarnings := run(t, flag)
				assert.Equal(t, table, legacy)
				assert.NotContains(t, legacy, "deprecated")
				assert.Empty(t, legacyWarnings)
			})
		}
	})
	t.Run("destinations match legacy", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "directory.yaml")
		file := filepath.Join(t.TempDir(), "file")
		legacy := filepath.Join(t.TempDir(), "legacy.yaml")
		_, warnings := run(t, "--output-dir", dir)
		assert.Empty(t, warnings)
		_, warnings = run(t, "--output-file", file)
		assert.Empty(t, warnings)
		legacyOutput, warnings := run(t, "-o", legacy)
		assert.Empty(t, warnings)
		// Cobra sends deprecation messages through its command output writer.
		assert.Contains(t, legacyOutput, "deprecated")
		assert.Contains(t, legacyOutput, "--output-dir or --output-file")
		expected, err := os.ReadFile(legacy)
		require.NoError(t, err)
		assert.Contains(t, string(expected), "output-test: \"true\"")
		for _, path := range []string{file, filepath.Join(dir, "output-example-mutated.yaml")} {
			content, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, string(expected), string(content))
		}
	})
}
