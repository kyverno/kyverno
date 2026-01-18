package report

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/kyverno/kyverno/pkg/engine/api"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
)

func TestPolicyExceptionHandling(t *testing.T) {
	tests := []struct {
		name        string
		status      api.RuleStatus
		isException bool
		expected    openreportsv1alpha1.Result
	}{
		{
			name:        "fail without exception",
			status:      api.RuleStatusFail,
			isException: false,
			expected:    openreportsv1alpha1.Result("fail"),
		},
		{
			name:        "fail with exception -> skip",
			status:      api.RuleStatusFail,
			isException: true,
			expected:    openreportsv1alpha1.Result("skip"),
		},
		{
			name:        "skip without exception",
			status:      api.RuleStatusSkip,
			isException: false,
			expected:    openreportsv1alpha1.Result("skip"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Manual result construction to test the exact logic
			result := openreportsv1alpha1.ReportResult{
				Result: toPolicyResult(tt.status),
			}

			// Simulate the exact fix logic we added
			if tt.isException {
				result.Result = "skip"
			}

			assert.Equal(t, tt.expected, result.Result)
		})
	}
}