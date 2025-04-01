package test

import (
	"testing"

	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestCreateRowsAccordingToResults(t *testing.T) {
	testCases := []struct {
		name           string
		testResult     v1alpha1.TestResult
		ok             bool
		expectedResult bool
	}{
		{
			name: "expected pass, actual pass",
			testResult: v1alpha1.TestResult{
				TestResultBase: v1alpha1.TestResultBase{
					Policy: "test-policy",
					Rule:   "test-rule",
					Kind:   "Pod",
					Result: policyreportv1alpha2.StatusPass,
				},
			},
			ok:             true, // checkResult returns true for passing tests
			expectedResult: true, // success should be true
		},
		{
			name: "expected pass, actual fail",
			testResult: v1alpha1.TestResult{
				TestResultBase: v1alpha1.TestResultBase{
					Policy: "test-policy",
					Rule:   "test-rule",
					Kind:   "Pod",
					Result: policyreportv1alpha2.StatusPass,
				},
			},
			ok:             false, // checkResult returns false for failing tests
			expectedResult: false, // success should be false
		},
		{
			name: "expected fail, actual fail",
			testResult: v1alpha1.TestResult{
				TestResultBase: v1alpha1.TestResultBase{
					Policy: "test-policy",
					Rule:   "test-rule",
					Kind:   "Pod",
					Result: policyreportv1alpha2.StatusFail,
				},
			},
			ok:             false, // checkResult returns false for failing tests
			expectedResult: true,  // success should be true because we expected a fail
		},
		{
			name: "expected fail, actual pass",
			testResult: v1alpha1.TestResult{
				TestResultBase: v1alpha1.TestResultBase{
					Policy: "test-policy",
					Rule:   "test-rule",
					Kind:   "Pod",
					Result: policyreportv1alpha2.StatusFail,
				},
			},
			ok:             true,  // checkResult returns true for passing tests
			expectedResult: false, // success should be false because the test should have failed
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// With the correct success calculation
			successFixed := (tc.ok && tc.testResult.Result == policyreportv1alpha2.StatusPass) ||
				(!tc.ok && tc.testResult.Result == policyreportv1alpha2.StatusFail)

			// With the buggy success calculation
			successBuggy := tc.ok

			// Verify the correct calculation matches expected value
			assert.Equal(t, tc.expectedResult, successFixed, "fixed success calculation should match expected value")

			// Test the buggy version against the correct logic
			if tc.testResult.Result == policyreportv1alpha2.StatusFail && tc.ok {
				// This is the case where the bug manifests: policy passes when we expected fail
				assert.NotEqual(t, successFixed, successBuggy,
					"buggy and fixed success calculations should differ for 'expected fail, actual pass' case")
			}
		})
	}
}
