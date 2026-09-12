package background

import (
	"testing"

	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestShouldKeepStaleResult_PolicyUnchanged(t *testing.T) {
	policyNameToLabel := map[string]string{
		"require-requests-limits": "policy.kyverno.io/require-requests-limits",
	}
	expected := map[string]string{"policy.kyverno.io/require-requests-limits": "1"}
	actual := map[string]string{"policy.kyverno.io/require-requests-limits": "1"}

	result := openreportsv1alpha1.ReportResult{Policy: "require-requests-limits"}

	assert.True(t, shouldKeepStaleResult(result, policyNameToLabel, expected, actual))
}

// Reproduces the scenario from the reported issue: a ClusterPolicy is edited to add
// `exclude.any.resources.namespaces: [kube-system]`, bumping its resource version. A stale
// "pass" result recorded for a kube-system Pod under the old version must be dropped so the
// resource is rescanned (and correctly excluded), instead of surviving in the report forever.
func TestShouldKeepStaleResult_PolicyChanged(t *testing.T) {
	policyNameToLabel := map[string]string{
		"require-requests-limits": "policy.kyverno.io/require-requests-limits",
	}
	expected := map[string]string{"policy.kyverno.io/require-requests-limits": "2"}
	actual := map[string]string{"policy.kyverno.io/require-requests-limits": "1"}

	result := openreportsv1alpha1.ReportResult{Policy: "require-requests-limits"}

	assert.False(t, shouldKeepStaleResult(result, policyNameToLabel, expected, actual))
}

func TestShouldKeepStaleResult_ExceptionChanged(t *testing.T) {
	policyNameToLabel := map[string]string{
		"require-requests-limits": "policy.kyverno.io/require-requests-limits",
		"default/my-exception":    "policy.kyverno.io/polexcp-default-my-exception",
	}
	expected := map[string]string{
		"policy.kyverno.io/require-requests-limits":      "1",
		"policy.kyverno.io/polexcp-default-my-exception": "2",
	}
	actual := map[string]string{
		"policy.kyverno.io/require-requests-limits":      "1",
		"policy.kyverno.io/polexcp-default-my-exception": "1",
	}

	result := openreportsv1alpha1.ReportResult{
		Policy: "require-requests-limits",
		Properties: map[string]string{
			"exceptions": "default/my-exception",
		},
	}

	assert.False(t, shouldKeepStaleResult(result, policyNameToLabel, expected, actual))
}

func TestShouldKeepStaleResult_NoMatchingLabel(t *testing.T) {
	result := openreportsv1alpha1.ReportResult{Policy: "some-other-policy"}

	assert.False(t, shouldKeepStaleResult(result, map[string]string{}, map[string]string{}, map[string]string{}))
}
