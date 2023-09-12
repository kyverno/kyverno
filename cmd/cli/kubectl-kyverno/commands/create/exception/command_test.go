package exception

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommand(t *testing.T) {
	cmd := Command()
	cmd.SetArgs([]string{"test", "--policy-rules", "policy,rule-1,rule-2"})
	err := cmd.Execute()
	assert.NoError(t, err)
}

func TestCommandWithMultipleArgs(t *testing.T) {
	cmd := Command()
	cmd.SetArgs([]string{"test", "test2", "--policy-rules", "policy,rule-1,rule-2"})
	err := cmd.Execute()
	assert.Error(t, err)
}

func TestCommandWithoutPolicyRules(t *testing.T) {
	cmd := Command()
	cmd.SetArgs([]string{"test", "test2"})
	err := cmd.Execute()
	assert.Error(t, err)
}

func TestCommandWithAny(t *testing.T) {
	cmd := Command()
	cmd.SetArgs([]string{"test", "--policy-rules", "policy,rule-1,rule-2", "--any", "kind=Pod,kind=Deployment,name=test-*"})
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	err := cmd.Execute()
	assert.NoError(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	expected := `
apiVersion: kyverno.io/v2alpha1
kind: PolicyException
metadata:
  name: test
  namespace: 
spec:
  background: true
  match:
    any:
      -
        kinds:
          - Pod
          - Deployment
        names:
          - test-*
  exceptions:
    - policyName: policy
      ruleNames:
        - rule-1
        - rule-2`
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(out)))
}

func TestCommandWithAll(t *testing.T) {
	cmd := Command()
	cmd.SetArgs([]string{"test", "--policy-rules", "policy,rule-1,rule-2", "--all", "kind=Pod,kind=Deployment,name=test-*,namespace=test,operation=UPDATE"})
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	err := cmd.Execute()
	assert.NoError(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	expected := `
apiVersion: kyverno.io/v2alpha1
kind: PolicyException
metadata:
  name: test
  namespace: 
spec:
  background: true
  match:
    all:
      -
        kinds:
          - Pod
          - Deployment
        names:
          - test-*
        namespaces:
          - test
        operations:
          - UPDATE
  exceptions:
    - policyName: policy
      ruleNames:
        - rule-1
        - rule-2`
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(out)))
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
	expected := `Error: accepts 1 arg(s), received 0`
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
