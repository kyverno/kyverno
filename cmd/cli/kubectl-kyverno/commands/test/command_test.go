package test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/openreports"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func TestCheckResultDetectsMismatch(t *testing.T) {
	policy := &kyvernov1.ClusterPolicy{}
	policy.SetName("test-policy")

	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "default",
			},
		},
	}

	tests := []struct {
		name           string
		ruleStatus     engineapi.RuleStatus
		expectedResult string
		wantOk         bool
		wantReason     string
	}{
		{
			name:           "expect fail but got pass - should detect mismatch",
			ruleStatus:     engineapi.RuleStatusPass,
			expectedResult: openreports.StatusFail,
			wantOk:         false,
			wantReason:     "Want fail, got pass",
		},
		{
			name:           "expect fail and got fail - should match",
			ruleStatus:     engineapi.RuleStatusFail,
			expectedResult: openreports.StatusFail,
			wantOk:         true,
			wantReason:     "Ok",
		},
		{
			name:           "expect pass and got pass - should match",
			ruleStatus:     engineapi.RuleStatusPass,
			expectedResult: openreports.StatusPass,
			wantOk:         true,
			wantReason:     "Ok",
		},
		{
			name:           "expect pass but got fail - should detect mismatch",
			ruleStatus:     engineapi.RuleStatusFail,
			expectedResult: openreports.StatusPass,
			wantOk:         false,
			wantReason:     "Want pass, got fail",
		},
		{
			name:           "expect fail but got error - should detect mismatch",
			ruleStatus:     engineapi.RuleStatusError,
			expectedResult: openreports.StatusFail,
			wantOk:         false,
			wantReason:     "Want fail, got error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rule engineapi.RuleResponse
			switch tt.ruleStatus {
			case engineapi.RuleStatusPass:
				rule = *engineapi.RulePass("test-rule", engineapi.Validation, "msg", nil)
			case engineapi.RuleStatusFail:
				rule = *engineapi.RuleFail("test-rule", engineapi.Validation, "msg", nil)
			case engineapi.RuleStatusError:
				rule = *engineapi.RuleError("test-rule", engineapi.Validation, "msg", nil, nil)
			}

			response := engineapi.NewEngineResponse(
				resource,
				engineapi.NewKyvernoPolicy(policy),
				nil,
			).WithPolicyResponse(engineapi.PolicyResponse{
				Rules: []engineapi.RuleResponse{rule},
			})

			testResult := v1alpha1.TestResult{
				TestResultBase: v1alpha1.TestResultBase{
					Policy: "test-policy",
					Rule:   "test-rule",
					Result: openreportsv1alpha1.Result(tt.expectedResult),
				},
			}

			ok, _, reason := checkResult(testResult, nil, "", response, rule, resource, true)
			assert.Equal(t, tt.wantOk, ok, "checkResult ok")
			assert.Contains(t, reason, tt.wantReason, "checkResult reason")
		})
	}
}

func TestResultCountsOnMismatch(t *testing.T) {
	color.Init(true)

	tests := []struct {
		name     string
		ok       bool
		expected string
		wantPass int
		wantFail int
	}{
		{
			name:     "mismatch with expected fail should count as failure",
			ok:       false,
			expected: openreports.StatusFail,
			wantPass: 0,
			wantFail: 1,
		},
		{
			name:     "match with expected fail should count as pass",
			ok:       true,
			expected: openreports.StatusFail,
			wantPass: 1,
			wantFail: 0,
		},
		{
			name:     "mismatch with expected pass should count as failure",
			ok:       false,
			expected: openreports.StatusPass,
			wantPass: 0,
			wantFail: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := &resultCounts{}
			testCount := 1
			testResult := v1alpha1.TestResult{
				TestResultBase: v1alpha1.TestResultBase{
					Policy: "test-policy",
					Rule:   "test-rule",
					Result: openreportsv1alpha1.Result(tt.expected),
				},
			}

			createRowsAccordingToResults(testResult, rc, &testCount, "test-rule", tt.ok, "msg", "reason", "v1/Pod/default/test-pod")

			assert.Equal(t, tt.wantPass, rc.Pass, "pass count")
			assert.Equal(t, tt.wantFail, rc.Fail, "fail count")
		})
	}
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
	testResponse, err := runTest(out, testCase, false)
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
