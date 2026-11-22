package test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	fatihcolor "github.com/fatih/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/stretchr/testify/assert"
)

func TestCommand(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	err := cmd.Execute()
	assert.Error(t, err)
}

func TestCommandInvalidFileName(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{"foo", "-f", ""})
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
	expected := `Error: requires at least 1 arg(s), only received 0`
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

func TestCommand_ColorAndRemoveColor(t *testing.T) {
	origNoColor := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = origNoColor
		color.Init(origNoColor)
	})

	testContent := `apiVersion: cli.kyverno.io/v1alpha1
kind: Test
policies:
  - policy.yaml
resources:
  - resource.yaml
results:
  - policy: my-policy
    rule: my-rule
    resources:
      - my-pod
    result: pass
`
	tmpDir := t.TempDir()
	testFilePath := tmpDir + "/kyverno-test.yaml"
	err := os.WriteFile(testFilePath, []byte(testContent), 0o600)
	assert.NoError(t, err)

	t.Run("color enabled by default", func(t *testing.T) {
		fatihcolor.NoColor = false
		color.Init(false)

		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{tmpDir})
		err := cmd.Execute()
		assert.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "\x1b[36;1m"+testFilePath+"\x1b[0;22m")
		assert.Contains(t, output, "\x1b[33;1mWARNING:\x1b[0;22m")
		assert.Contains(t, output, "\x1b[32;1mDone.\x1b[0;22m")
	})

	t.Run("remove color with flag", func(t *testing.T) {
		fatihcolor.NoColor = false
		color.Init(false)

		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{tmpDir, "--remove-color"})
		err := cmd.Execute()
		assert.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, testFilePath)
		assert.Contains(t, output, "WARNING: name is not set")
		assert.Contains(t, output, "Done.")
		assert.NotContains(t, output, "\x1b[")
	})
}
