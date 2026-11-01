package values

import (
	"bytes"
	"io"
	"strings"
	"testing"

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
	expected := `Error: unknown command "foo" for "values"`
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

func TestCommandAggregationAndValuesWithEquals(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetArgs([]string{
		"-g", "request.mode=dev",
		"-g", "token=YWJjZGVmZ2hpag==",
		"-n", "prod,env=prod",
		"-n", "prod,tier=backend",
		"--rule", "my-policy,check-labels,app=frontend",
		"--rule", "my-policy,check-probes,readiness=true",
		"--resource", "my-policy,pod1,tier=web",
		"--rule", "other-policy,other-rule,k=v",
	})
	err := cmd.Execute()
	assert.NoError(t, err)

	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	outputStr := string(out)

	// Verify global values (including base64 with '=' sign preserved)
	assert.Contains(t, outputStr, "request.mode: dev")
	assert.Contains(t, outputStr, "token: YWJjZGVmZ2hpag==")

	// Verify namespaceSelector aggregation
	assert.Contains(t, outputStr, "- name: prod")
	assert.Contains(t, outputStr, "env: prod")
	assert.Contains(t, outputStr, "tier: backend")

	// Verify policy aggregation under single - name: my-policy
	assert.Equal(t, 1, strings.Count(outputStr, "- name: my-policy"))
	assert.Contains(t, outputStr, "- name: check-labels")
	assert.Contains(t, outputStr, "- name: check-probes")
	assert.Contains(t, outputStr, "- name: pod1")

	// Verify other-policy is also present
	assert.Equal(t, 1, strings.Count(outputStr, "- name: other-policy"))
	assert.Contains(t, outputStr, "- name: other-rule")
}

func TestCommandValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "invalid global value without equals",
			args: []string{"-g", "invalid_no_equals"},
		},
		{
			name: "invalid global value with empty key",
			args: []string{"-g", "=value"},
		},
		{
			name: "invalid namespace selector single part",
			args: []string{"-n", "only_namespace"},
		},
		{
			name: "invalid namespace selector invalid label",
			args: []string{"-n", "prod,invalid_label"},
		},
		{
			name: "invalid rule single part",
			args: []string{"--rule", "only_policy"},
		},
		{
			name: "invalid rule invalid key value",
			args: []string{"--rule", "pol,rule,invalid_key_value"},
		},
		{
			name: "invalid resource single part",
			args: []string{"--resource", "only_policy"},
		},
		{
			name: "invalid resource invalid key value",
			args: []string{"--resource", "pol,res,invalid_key_value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := Command()
			assert.NotNil(t, cmd)
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			assert.Error(t, err)
		})
	}
}
