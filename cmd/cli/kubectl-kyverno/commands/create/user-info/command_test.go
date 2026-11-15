package userinfo

import (
	"bytes"
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
