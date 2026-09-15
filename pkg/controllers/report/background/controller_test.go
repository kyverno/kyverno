package background

import (
	"testing"

	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/stretchr/testify/assert"
)

// Test_keepKyvernoPolicyResult covers when a policy's
// matchConstraints/resourceRules change (its resourceVersion bumps) so that it
// no longer targets a given resource, a previously stored ReportResult for that
// policy against that resource must NOT be blindly retained on a partial
// reconcile. If it is, the stale result (and the PolicyReport built from it)
// never gets cleaned up.
func Test_keepKyvernoPolicyResult(t *testing.T) {
	const label = "audit.kyverno.io/validatingpolicy.debug-change-in-target-type"

	tests := []struct {
		name              string
		result            openreportsv1alpha1.ReportResult
		policyNameToLabel map[string]string
		expected          map[string]string
		actual            map[string]string
		want              bool
	}{
		{
			name:              "policy unchanged -> result is kept",
			result:            openreportsv1alpha1.ReportResult{Policy: "debug-change-in-target-type"},
			policyNameToLabel: map[string]string{"debug-change-in-target-type": label},
			expected:          map[string]string{label: "13050"},
			actual:            map[string]string{label: "13050"},
			want:              true,
		},
		{
			name:              "policy changed (matchConstraints updated) -> stale result is dropped",
			result:            openreportsv1alpha1.ReportResult{Policy: "debug-change-in-target-type"},
			policyNameToLabel: map[string]string{"debug-change-in-target-type": label},
			expected:          map[string]string{label: "13109"},
			actual:            map[string]string{label: "13050"},
			want:              false,
		},
		{
			name:              "policy no longer exists -> result is dropped",
			result:            openreportsv1alpha1.ReportResult{Policy: "deleted-policy"},
			policyNameToLabel: map[string]string{},
			expected:          map[string]string{},
			actual:            map[string]string{},
			want:              false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keepKyvernoPolicyResult(tt.result, tt.policyNameToLabel, tt.expected, tt.actual)
			assert.Equal(t, tt.want, got)
		})
	}
}