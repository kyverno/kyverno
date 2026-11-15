package userinfo

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/fix"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestCommand(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	err := cmd.Execute()
	assert.NoError(t, err)

	out, err := io.ReadAll(b)
	assert.NoError(t, err)

	var userInfo v1alpha1.UserInfo
	err = yaml.UnmarshalStrict(out, &userInfo)
	assert.NoError(t, err)
	assert.Equal(t, "cli.kyverno.io/v1alpha1", userInfo.APIVersion)
	assert.Equal(t, "UserInfo", userInfo.Kind)
	assert.Equal(t, "user-info", userInfo.Name)
}

func TestCommandWithFlags(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetArgs([]string{
		"--name", "custom-user",
		"--username", "alice",
		"--group", "devs,admins",
		"--role", "edit",
		"--cluster-role", "view",
	})
	err := cmd.Execute()
	require.NoError(t, err)

	out, err := io.ReadAll(b)
	require.NoError(t, err)

	var userInfo v1alpha1.UserInfo
	err = yaml.UnmarshalStrict(out, &userInfo)
	require.NoError(t, err)
	assert.Equal(t, "cli.kyverno.io/v1alpha1", userInfo.APIVersion)
	assert.Equal(t, "UserInfo", userInfo.Kind)
	assert.Equal(t, "custom-user", userInfo.Name)
	assert.Equal(t, "alice", userInfo.AdmissionUserInfo.Username)
	assert.Equal(t, []string{"devs", "admins"}, userInfo.AdmissionUserInfo.Groups)
	assert.Equal(t, []string{"edit"}, userInfo.Roles)
	assert.Equal(t, []string{"view"}, userInfo.ClusterRoles)

	// Verify fix.FixUserInfo produces zero warnings on the generated resource
	_, warnings, err := fix.FixUserInfo(userInfo)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestCommandOutputFile(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "userinfo.yaml")

	cmd := Command()
	cmd.SetArgs([]string{
		"--output", outPath,
		"-n", "file-user",
		"-u", "bob",
		"-g", "testers",
	})
	err := cmd.Execute()
	require.NoError(t, err)

	refPath := filepath.Join(tmpDir, "reference")
	require.NoError(t, os.WriteFile(refPath, []byte(""), 0o644))
	refInfo, err := os.Stat(refPath)
	require.NoError(t, err)

	info, err := os.Stat(outPath)
	require.NoError(t, err)
	assert.Equal(t, refInfo.Mode().Perm(), info.Mode().Perm())

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)

	var userInfo v1alpha1.UserInfo
	err = yaml.UnmarshalStrict(data, &userInfo)
	require.NoError(t, err)
	assert.Equal(t, "cli.kyverno.io/v1alpha1", userInfo.APIVersion)
	assert.Equal(t, "UserInfo", userInfo.Kind)
	assert.Equal(t, "file-user", userInfo.Name)
	assert.Equal(t, "bob", userInfo.AdmissionUserInfo.Username)
	assert.Equal(t, []string{"testers"}, userInfo.AdmissionUserInfo.Groups)
}

func TestCommandWithArgs(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{"foo"})
	err := cmd.Execute()
	assert.Error(t, err)
}

func TestCommandWithInvalidArg(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	cmd.SetArgs([]string{"foo"})
	err := cmd.Execute()
	assert.Error(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	expected := `Error: unknown command "foo" for "user-info"`
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

func TestCommandNameYAMLValues(t *testing.T) {
	testCases := []string{
		"no",
		"off",
		"yes",
		"on",
		"true",
		"false",
		"null",
	}
	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			cmd := Command()
			b := bytes.NewBufferString("")
			cmd.SetOut(b)
			cmd.SetArgs([]string{"--name", tc})
			err := cmd.Execute()
			require.NoError(t, err)

			out, err := io.ReadAll(b)
			require.NoError(t, err)

			var userInfo v1alpha1.UserInfo
			err = yaml.UnmarshalStrict(out, &userInfo)
			require.NoError(t, err)
			assert.Equal(t, tc, userInfo.Name)
		})
	}
}

func TestCommandInvalidName(t *testing.T) {
	invalidNames := []string{
		"Alice_1",
		"my user",
		"user:with:colons",
		"",
		"-starts-with-dash",
		"ends-with-dash-",
		"contains_underscore",
		"UPPERCASE",
		strings.Repeat("a", 254),
	}
	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			tmpDir := t.TempDir()
			outPath := filepath.Join(tmpDir, "userinfo.yaml")
			cmd := Command()
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{"--name", name, "--output", outPath})
			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), fmt.Sprintf("invalid name %q", name))

			// Verify file was never created because validation failed before rendering
			_, err = os.Stat(outPath)
			assert.True(t, os.IsNotExist(err))
		})
	}
}

func TestCommandAllFieldsYAMLTypeCoercion(t *testing.T) {
	cmd := Command()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetArgs([]string{
		"--name", "no",
		"--username", "null",
		"--role", "true",
		"--cluster-role", "12345",
		"--group", "off",
	})
	err := cmd.Execute()
	require.NoError(t, err)

	out, err := io.ReadAll(b)
	require.NoError(t, err)

	var userInfo v1alpha1.UserInfo
	err = yaml.UnmarshalStrict(out, &userInfo)
	require.NoError(t, err)
	assert.Equal(t, "no", userInfo.Name)
	assert.Equal(t, "null", userInfo.AdmissionUserInfo.Username)
	assert.Equal(t, []string{"true"}, userInfo.Roles)
	assert.Equal(t, []string{"12345"}, userInfo.ClusterRoles)
	assert.Equal(t, []string{"off"}, userInfo.AdmissionUserInfo.Groups)
}

type failingWriter struct{}

func (f *failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failure")
}

func TestCommandFailingStdoutWriter(t *testing.T) {
	cmd := Command()
	cmd.SetOut(&failingWriter{})
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write failure")
}

func TestCommandInvalidOutputDestination(t *testing.T) {
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "non-existent-directory", "sub-dir", "userinfo.yaml")

	cmd := Command()
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--output", invalidPath})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestCommandTemplateExecutionFailure(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "userinfo.yaml")
	initialContent := []byte("existing content")
	err := os.WriteFile(outPath, initialContent, 0o644)
	require.NoError(t, err)

	cmd := commandWithTemplate("{{ .Name.InvalidField }}")
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--output", outPath})
	err = cmd.Execute()
	require.Error(t, err)

	// Verify the existing output file retains its contents (render-before-write guarantee)
	data, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Equal(t, initialContent, data)
}

func TestCommandTemplateParseFailure(t *testing.T) {
	cmd := commandWithTemplate("{{ unclosed")
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	require.Error(t, err)
}

func TestCommandEmptyListsOmitted(t *testing.T) {
	cmd := Command()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	err := cmd.Execute()
	require.NoError(t, err)

	out, err := io.ReadAll(b)
	require.NoError(t, err)

	outStr := string(out)
	assert.NotContains(t, outStr, "roles:")
	assert.NotContains(t, outStr, "clusterRoles:")
	assert.NotContains(t, outStr, "groups:")

	var userInfo v1alpha1.UserInfo
	err = yaml.UnmarshalStrict(out, &userInfo)
	require.NoError(t, err)
	assert.Empty(t, userInfo.Roles)
	assert.Empty(t, userInfo.ClusterRoles)
	assert.Empty(t, userInfo.AdmissionUserInfo.Groups)
}
