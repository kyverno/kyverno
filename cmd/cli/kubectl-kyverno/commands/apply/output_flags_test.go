package apply

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRejectFormatKeywordsAsOutputPath(t *testing.T) {
	tempDir := t.TempDir()
	origWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer func() {
		_ = os.Chdir(origWd)
	}()

	policyPath := filepath.Join(tempDir, "policy.yaml")
	resourcePath := filepath.Join(tempDir, "resource.yaml")
	require.NoError(t, os.WriteFile(policyPath, []byte(fmt.Sprintf(outputTestMutatingPolicy, `"team-a"`)), 0o600))
	require.NoError(t, os.WriteFile(resourcePath, []byte(outputTestPod), 0o600))

	formatKeywords := []string{"json", "yaml", "yml", "table"}
	for _, kw := range formatKeywords {
		t.Run("flag -o "+kw, func(t *testing.T) {
			cmd := Command()
			out := &bytes.Buffer{}
			cmd.SetOut(out)
			cmd.SetErr(out)
			cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "-o", kw})

			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), fmt.Sprintf("invalid path %q: did you mean --output-format %s?", kw, kw))

			// Crucial check: verify no directory or file named kw was created
			_, statErr := os.Stat(filepath.Join(tempDir, kw))
			assert.True(t, os.IsNotExist(statErr), "file or directory %q should not have been created on disk", kw)
		})

		t.Run("flag --output-dir "+kw, func(t *testing.T) {
			cmd := Command()
			out := &bytes.Buffer{}
			cmd.SetOut(out)
			cmd.SetErr(out)
			cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--output-dir", kw})

			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), fmt.Sprintf("invalid path %q: did you mean --output-format %s?", kw, kw))

			_, statErr := os.Stat(filepath.Join(tempDir, kw))
			assert.True(t, os.IsNotExist(statErr), "file or directory %q should not have been created on disk", kw)
		})
	}
}

func TestOutputDirMutatedPolicy(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	resourcePath := filepath.Join(dir, "resource.yaml")
	outputPath := filepath.Join(dir, "mutated.yaml")
	require.NoError(t, os.WriteFile(policyPath, []byte(fmt.Sprintf(outputTestMutatingPolicy, `"team-b"`)), 0o600))
	require.NoError(t, os.WriteFile(resourcePath, []byte(outputTestPod), 0o600))

	cmd := Command()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--output-dir", outputPath})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, out.String(), "pass: 1, fail: 0, warn: 0, error: 0, skip: 0")
	written, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Contains(t, string(written), "kind: Pod")
	assert.Contains(t, string(written), "team: team-b")
}

func TestOutputFormatTable(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	resourcePath := filepath.Join(dir, "resource.yaml")
	require.NoError(t, os.WriteFile(policyPath, []byte(fmt.Sprintf(outputTestMutatingPolicy, `"team-c"`)), 0o600))
	require.NoError(t, os.WriteFile(resourcePath, []byte(outputTestPod), 0o600))

	for _, flagArgs := range [][]string{
		{"--output-format", "table"},
		{"-t"},
		{"--table"},
	} {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		args := append([]string{policyPath, "--resource", resourcePath}, flagArgs...)
		cmd.SetArgs(args)
		err := cmd.Execute()
		require.NoError(t, err)

		outputStr := out.String()
		assert.Contains(t, outputStr, "POLICY", "table header should be printed")
		assert.Contains(t, outputStr, "RESOURCE", "table header should be printed")
		assert.Contains(t, outputStr, "RESULT", "table header should be printed")
	}
}

func TestOutputFormatConflicts(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	resourcePath := filepath.Join(dir, "resource.yaml")
	require.NoError(t, os.WriteFile(policyPath, []byte(fmt.Sprintf(outputTestMutatingPolicy, `"team-d"`)), 0o600))
	require.NoError(t, os.WriteFile(resourcePath, []byte(outputTestPod), 0o600))

	t.Run("table output with policy report flag -p -t", func(t *testing.T) {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "-p", "-t"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table output format cannot be used with -p/--policy-report")
	})

	t.Run("table output with policy report flag -p --output-format table", func(t *testing.T) {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "-p", "--output-format", "table"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table output format cannot be used with -p/--policy-report")
	})

	t.Run("table output with generate-exceptions", func(t *testing.T) {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--generate-exceptions", "--output-format", "table"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table output format cannot be used with --generate-exceptions")
	})

	t.Run("conflicting --table and --output-format json", func(t *testing.T) {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--table", "--output-format", "json"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflicting output formats: --table and --output-format json cannot be used together")
	})

	t.Run("invalid output-format", func(t *testing.T) {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--output-format", "unsupported"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid output format "unsupported" (valid formats: table, json, yaml)`)
	})

	t.Run("conflicting --output and --output-dir flags", func(t *testing.T) {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--output", filepath.Join(dir, "dir1.yaml"), "--output-dir", filepath.Join(dir, "dir2.yaml")})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot use both --output and --output-dir")
	})
}

func TestOutputFormatWarningWithoutReport(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	resourcePath := filepath.Join(dir, "resource.yaml")
	require.NoError(t, os.WriteFile(policyPath, []byte(fmt.Sprintf(outputTestMutatingPolicy, `"team-e"`)), 0o600))
	require.NoError(t, os.WriteFile(resourcePath, []byte(outputTestPod), 0o600))

	cmd := Command()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--output-format", "json"})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, stderr.String(), "WARNING: --output-format json has no effect without -p/--policy-report or --generate-exceptions")
	assert.Contains(t, stdout.String(), "pass: 1, fail: 0, warn: 0, error: 0, skip: 0")
}
