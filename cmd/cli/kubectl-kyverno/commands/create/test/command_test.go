package test

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/deprecations"
	clitest "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	"github.com/stretchr/testify/assert"
)

func TestCommand(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	err := cmd.Execute()
	assert.NoError(t, err)
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
	expected := `Error: unknown command "foo" for "test"`
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

func TestCommandOutput(t *testing.T) {
	for _, tc := range []struct {
		name, status, namespace, patched string
	}{
		{"documented pass", "pass", "resource-namespace", ""},
		{"patched pass", "pass", "resource-namespace", "patched.yaml"},
		{"fail", "fail", "resource-namespace", "patched.yaml"},
		{"skip", "skip", "resource-namespace", "patched.yaml"},
		{"no namespace", "pass", "", ""},
		{"metadata only", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "kyverno-test.yaml")
			args := []string{"-p", "policy.yaml", "-r", "resource.yaml", "-f", "values.yaml", "-o", file}
			if tc.status != "" {
				result := "policy-name,rule-name,resource-name," + tc.namespace + ",resource-kind"
				if tc.patched != "" {
					result += "," + tc.patched
				}
				args = append(args, "--"+tc.status, result)
			}
			cmd := Command()
			cmd.SetArgs(args)
			assert.NoError(t, cmd.Execute())
			tests := clitest.LoadTest(nil, file)
			if !assert.Len(t, tests, 1) || !assert.NoError(t, tests[0].Err) || !assert.NotNil(t, tests[0].Test) {
				return
			}
			loaded := tests[0].Test
			assert.False(t, deprecations.CheckTest(io.Discard, file, loaded))
			assert.Equal(t, "cli.kyverno.io/v1alpha1", loaded.APIVersion)
			assert.Equal(t, "Test", loaded.Kind)
			assert.Equal(t, "test-name", loaded.ObjectMeta.Name)
			if tc.status == "" || !assert.Len(t, loaded.Results, 1) {
				return
			}
			result := loaded.Results[0]
			assert.Equal(t, []string{strings.TrimPrefix(tc.namespace+"/resource-name", "/")}, result.Resources)
			assert.Equal(t, "resource-kind", result.Kind)
			assert.Equal(t, "rule-name", result.Rule)
			assert.Equal(t, "policy-name", result.Policy)
			assert.Equal(t, tc.status, string(result.Result))
			assert.Equal(t, tc.patched, result.PatchedResources)
		})
	}
}
