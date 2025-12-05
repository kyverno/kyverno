package test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestCommandNoTests(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	errBuffer := bytes.NewBufferString("")
	cmd.SetErr(errBuffer)
	outBuffer := bytes.NewBufferString("")
	cmd.SetOut(outBuffer)
	cmd.SetArgs([]string{"."})
	err := cmd.Execute()
	assert.NoError(t, err)
	out, err := io.ReadAll(outBuffer)
	assert.NoError(t, err)
	expected := `No test yamls available`
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(out)))
	errOut, err := io.ReadAll(errBuffer)
	assert.NoError(t, err)
	expected = ``
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(errOut)))
}

func TestCommandRequireTests(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	errBuffer := bytes.NewBufferString("")
	cmd.SetErr(errBuffer)
	outBuffer := bytes.NewBufferString("")
	cmd.SetOut(outBuffer)
	cmd.SetArgs([]string{".", "--require-tests"})
	err := cmd.Execute()
	assert.Error(t, err)
	out, err := io.ReadAll(outBuffer)
	assert.NoError(t, err)
	expected := `No test yamls available`
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(out)))
	errOut, err := io.ReadAll(errBuffer)
	assert.NoError(t, err)
	expected = `Error: no tests found`
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(errOut)))
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

func Test_JSONPayload(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err, "Failed to get working directory")
	rootDir := filepath.Join(wd, "..", "..", "..", "..", "..")
	testDir := filepath.Join(rootDir, "test", "cli", "test-validating-policy", "json-check-dockerfile")

	_, err = os.Stat(testDir)
	if os.IsNotExist(err) {
		t.Skip("Test directory not found, skipping test")
		return
	}

	testFile := filepath.Join(testDir, "kyverno-test.yaml")
	testCases := test.LoadTest(nil, testFile)
	require.Len(t, testCases, 1, "Expected exactly one test case in %s", testFile)

	testCase := testCases[0]

	out := &bytes.Buffer{}
	t.Logf("Running test with files from %s", testCase.Dir())
	testResponse, err := runTest(out, testCase, false, false)
	require.NoError(t, err, "Failed to run test")

	t.Logf("Test output: %s", out.String())

	t.Run("Check policy results match output table", func(t *testing.T) {
		payloadKey := testCase.Test.JSONPayload
		responses := testResponse.Trigger[payloadKey]
		policyResults := make(map[string]struct {
			Status string
			Reason string
		})

		for _, response := range responses {
			policy := response.Policy().GetName()

			// Determine overall result - based on the table, all policies are passing
			status := "pass"
			reason := "Ok"

			policyResults[policy] = struct {
				Status string
				Reason string
			}{
				Status: status,
				Reason: reason,
			}
		}

		wgetResult, found := policyResults["check-dockerfile-disallow-wget"]
		if assert.True(t, found, "wget policy result not found") {
			assert.Equal(t, "pass", wgetResult.Status, "wget policy should pass according to output table")
			assert.Equal(t, "Ok", wgetResult.Reason, "wget policy reason should be 'Ok'")
		}

		curlResult, found := policyResults["check-dockerfile-disallow-curl"]
		if assert.True(t, found, "curl policy result not found") {
			assert.Equal(t, "pass", curlResult.Status, "curl policy should pass according to output table")
			assert.Equal(t, "Ok", curlResult.Reason, "curl policy reason should be 'Ok'")
		}
	})
}

func Test_DefaultAllowExistingViolations(t *testing.T) {
	tmpDir := t.TempDir()

	policy := `
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: disallow-latest-tag
spec:
  validationFailureAction: Enforce
  rules:
  - match:
      any:
      - resources:
          kinds:
          - Pod
    name: validate-image-tag
    validate:
      message: Using a mutable image tag e.g. 'latest' is not allowed
      pattern:
        spec:
          containers:
          - image: '!*:latest'
`
	err := os.WriteFile(filepath.Join(tmpDir, "policy.yaml"), []byte(policy), 0644)
	require.NoError(t, err)

	resource := `
apiVersion: v1
kind: Pod
metadata:
  name: myapp-pod
  labels:
    app: myapp
spec:
  containers:
  - name: nginx
    image: nginx:latest
`
	err = os.WriteFile(filepath.Join(tmpDir, "resource.yaml"), []byte(resource), 0644)
	require.NoError(t, err)

	testYaml := `
name: test-allow-existing
policies:
  - policy.yaml
resources:
  - resource.yaml
values:
  globalValues:
    request.operation: UPDATE
results:
  - policy: disallow-latest-tag
    rule: validate-image-tag
    resources:
      - myapp-pod
    kind: Pod
    result: fail
`
	err = os.WriteFile(filepath.Join(tmpDir, "kyverno-test.yaml"), []byte(testYaml), 0644)
	require.NoError(t, err)

	testCases := test.LoadTest(nil, filepath.Join(tmpDir, "kyverno-test.yaml"))
	require.Len(t, testCases, 1)
	testCase := testCases[0]

	out := &bytes.Buffer{}

	// Run with false (default) -> Should Fail
	resp, err := runTest(out, testCase, false, false)
	require.NoError(t, err)

	found := false
	for _, ers := range resp.Trigger {
		for _, er := range ers {
			for _, rule := range er.PolicyResponse.Rules {
				if rule.Name() == "validate-image-tag" {
					found = true
					assert.Equal(t, engineapi.RuleStatusFail, rule.Status(), "Expected Fail when defaultAllowExistingViolations is false")
				}
			}
		}
	}
	assert.True(t, found, "Rule result not found for false case")

	// Run with true -> Should Skip
	respTrue, err := runTest(out, testCase, false, true)
	require.NoError(t, err)

	found = false
	for _, ers := range respTrue.Trigger {
		for _, er := range ers {
			for _, rule := range er.PolicyResponse.Rules {
				if rule.Name() == "validate-image-tag" {
					found = true
					assert.Equal(t, engineapi.RuleStatusSkip, rule.Status(), "Expected Skip when defaultAllowExistingViolations is true")
				}
			}
		}
	}
	assert.True(t, found, "Rule result not found for true case")
}
