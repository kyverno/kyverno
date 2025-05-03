package test

import (
	"testing"

	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestSuccessCalculation(t *testing.T) {
	tests := []struct {
		name           string
		ok             bool
		expectedResult policyreportv1alpha2.PolicyResult
		wantSuccess    bool
	}{
		{
			name:           "ValidationSuccess_ExpectedPass",
			ok:             true,
			expectedResult: policyreportv1alpha2.StatusPass,
			wantSuccess:    true,
		},
		{
			name:           "ValidationFail_ExpectedPass",
			ok:             false,
			expectedResult: policyreportv1alpha2.StatusPass,
			wantSuccess:    false,
		},
		{
			name:           "ValidationFail_ExpectedFail",
			ok:             false,
			expectedResult: policyreportv1alpha2.StatusFail,
			wantSuccess:    true,
		},
		{
			name:           "ValidationSuccess_ExpectedFail",
			ok:             true,
			expectedResult: policyreportv1alpha2.StatusFail,
			wantSuccess:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			test := v1alpha1.TestResult{
				TestResultBase: v1alpha1.TestResultBase{
					Result: tt.expectedResult,
				},
			}
			success := (tt.ok && test.Result == policyreportv1alpha2.StatusPass) || (!tt.ok && test.Result == policyreportv1alpha2.StatusFail)
			assert.Equal(t, tt.wantSuccess, success)
		})
	}
}
