package apply

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJSONMutationApplyOutput(t *testing.T) {
	dir := ".json-mutation-output-test"
	require.NoError(t, os.Mkdir(dir, 0o700))
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(dir)) })
	fixtures := "../../../../../test/cli/test-mutating-policy/"
	config := ApplyCommandConfig{
		PolicyPaths:   []string{fixtures + "json-document-mutation/policy.yaml"},
		JSONPaths:     []string{fixtures + "json-document-mutation/input.json"},
		MutateLogPath: filepath.Join(dir, "output.json"),
	}
	var out bytes.Buffer
	rc, _, _, _, err := config.applyCommandHelper(context.Background(), &out)
	require.NoError(t, err)
	require.Equal(t, 2, rc.Pass)
	content, err := os.ReadFile(config.MutateLogPath)
	require.NoError(t, err)
	require.Contains(t, string(content), "9007199254740993")
	require.Contains(t, string(content), `"region": "west"`)

	config.JSONPaths = append(config.JSONPaths, fixtures+"json-document-mutation/nested/input.json")
	config.MutateLogPath = filepath.Join(dir, "documents")
	_, _, _, _, err = config.applyCommandHelper(context.Background(), &out)
	require.NoError(t, err)
	files, err := os.ReadDir(config.MutateLogPath)
	require.NoError(t, err)
	require.Len(t, files, 2, "same basenames in distinct paths must not collide")
	for _, f := range files {
		require.Regexp(t, `^input-[0-9a-f]{8}-mutated\.json$`, f.Name())
	}

	config.PolicyPaths = []string{fixtures + "json-skips-errors/policy.yaml"}
	config.JSONPaths = []string{fixtures + "json-skips-errors/input.json"}
	config.MutateLogPath = filepath.Join(dir, "preserved.json")
	require.NoError(t, os.WriteFile(config.MutateLogPath, []byte("original"), 0o600))
	_, _, _, _, err = config.applyCommandHelper(context.Background(), &out)
	require.Error(t, err)
	content, err = os.ReadFile(config.MutateLogPath)
	require.NoError(t, err)
	require.Equal(t, "original", string(content), "failed mutation must not truncate output")

	config.MutateLogPath = filepath.Join(dir, "never-created")
	_, _, _, _, err = config.applyCommandHelper(context.Background(), &out)
	require.Error(t, err)
	_, err = os.Stat(config.MutateLogPath)
	require.True(t, os.IsNotExist(err))

	config.PolicyPaths = []string{fixtures + "json-root-transitions/policy.yaml"}
	config.JSONPaths = []string{fixtures + "json-root-transitions/input.json"}
	config.MutateLogPath = ""
	out.Reset()
	rc, _, _, _, err = config.applyCommandHelper(context.Background(), &out)
	require.NoError(t, err)
	require.Equal(t, 3, rc.Pass)
	require.Contains(t, out.String(), "\nnull\n", "the default output must include literal null")

	config.PolicyPaths = []string{fixtures + "json-document-mutation/policy.yaml"}
	config.JSONPaths = []string{fixtures + "json-root-transitions/input.json"}
	config.MutateLogPath = filepath.Join(dir, "invalid-root.json")
	_, _, _, _, err = config.applyCommandHelper(context.Background(), &out)
	require.Error(t, err)
	_, err = os.Stat(config.MutateLogPath)
	require.True(t, os.IsNotExist(err))

	// A later input failing after an earlier one succeeded must not write any
	// staged output, to either a file or a directory.
	config.JSONPaths = []string{fixtures + "json-document-mutation/input.json", fixtures + "json-root-transitions/input.json"}
	config.MutateLogPath = filepath.Join(dir, "staged.json")
	require.NoError(t, os.WriteFile(config.MutateLogPath, []byte("original"), 0o600))
	_, _, _, _, err = config.applyCommandHelper(context.Background(), &out)
	require.Error(t, err)
	content, err = os.ReadFile(config.MutateLogPath)
	require.NoError(t, err)
	require.Equal(t, "original", string(content), "partial multi-input output must not be written")
	config.MutateLogPath = filepath.Join(dir, "staged-dir")
	require.NoError(t, os.Mkdir(config.MutateLogPath, 0o700))
	_, _, _, _, err = config.applyCommandHelper(context.Background(), &out)
	require.Error(t, err)
	files, err = os.ReadDir(config.MutateLogPath)
	require.NoError(t, err)
	require.Empty(t, files, "partial multi-input output must not be written to a directory")

	invalid := filepath.Join(dir, "invalid.json")
	require.NoError(t, os.WriteFile(invalid, []byte("null null"), 0o600))
	config.JSONPaths = []string{invalid}
	_, _, _, _, err = config.applyCommandHelper(context.Background(), &out)
	require.ErrorContains(t, err, "invalid JSON document")

	contents, err := os.ReadFile(fixtures + "json-document-mutation/policy.yaml")
	require.NoError(t, err)
	denyingPolicy := filepath.Join(dir, "deny.yaml")
	require.NoError(t, os.WriteFile(denyingPolicy, bytes.ReplaceAll(contents, []byte(`object.region == "west"`), []byte("false")), 0o600))
	config.PolicyPaths = []string{denyingPolicy}
	config.JSONPaths = []string{fixtures + "json-document-mutation/input.json"}
	config.MutateLogPath = filepath.Join(dir, "denied.json")
	require.NoError(t, os.WriteFile(config.MutateLogPath, []byte("original"), 0o600))
	rc, _, _, _, err = config.applyCommandHelper(context.Background(), &out)
	require.NoError(t, err)
	require.Equal(t, 1, rc.Fail)
	content, err = os.ReadFile(config.MutateLogPath)
	require.NoError(t, err)
	require.Equal(t, "original", string(content), "failed validation must not write partial mutation output")
}
