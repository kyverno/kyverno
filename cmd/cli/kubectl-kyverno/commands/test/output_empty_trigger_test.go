package test

import (
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/table"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/openreports"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPrintTestResult_GenerateRuleEmptyTriggerIsSkipNotFail is a regression test for
// https://github.com/kyverno/kyverno/issues/8942 ("[Bug] [CLI] Negative tests for
// generate policy with exclusions fail").
//
// For a generate/mutateExisting rule whose selector excludes a resource entirely, the
// engine/CLI evaluation records the resource's key in TestResponse.Trigger with an
// EMPTY []engineapi.EngineResponse slice: the map key exists (the resource was
// considered), but zero EngineResponse values were appended for it, because no rule
// ever fired. printTestResult must still treat that as "no evidence of a rule firing on
// this resource" and report it as Skip, not Fail.
func TestPrintTestResult_GenerateRuleEmptyTriggerIsSkipNotFail(t *testing.T) {
	color.Init(true)

	resourceKey := "v1,Pod,default,test-pod"

	tests := []v1alpha1.TestResult{
		{
			TestResultBase: v1alpha1.TestResultBase{
				Policy: "test-policy",
				Rule:   "test-rule",
				Result: openreportsv1alpha1.Result(openreports.StatusSkip),
			},
		},
	}

	responses := &TestResponse{
		// Key present, zero responses: what the real engine/CLI path produces for a
		// generate rule whose match/exclude selector did not select the resource at
		// all -- see issue #8942. This is NOT the same as the key being absent.
		Trigger: map[string][]engineapi.EngineResponse{
			resourceKey: {},
		},
	}

	rc := &resultCounts{}
	resultsTable := &table.Table{}

	err := printTestResult(tests, responses, rc, resultsTable, nil, "", true)
	require.NoError(t, err)

	require.Len(t, resultsTable.RawRows, 1, "expected exactly one result row for the resource")
	row := resultsTable.RawRows[0]

	assert.False(t, row.IsFailure, "a resource with zero rule responses must not be reported as a failure")
	assert.Equal(t, 1, rc.Skip, "expected the resource to be counted as Skip")
	assert.Equal(t, 0, rc.Fail, "expected no failures")
	assert.Equal(t, 0, rc.Pass, "this code path increments Skip, not Pass")
}
