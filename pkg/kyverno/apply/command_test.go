package apply

import (
	"testing"

	preport "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	"gotest.tools/assert"
)

func Test_Apply(t *testing.T) {
	type TestCase struct {
		PolicyPaths           []string
		ResourcePaths         []string
		expectedPolicyReports []preport.PolicyReport
	}

	testcases := []TestCase{
		TestCase{
			PolicyPaths:   []string{"../../../samples/best_practices/disallow_latest_tag.yaml"},
			ResourcePaths: []string{"../../../test/resources/pod_with_version_tag.yaml"},
			expectedPolicyReports: []preport.PolicyReport{
				preport.PolicyReport{
					Summary: preport.PolicyReportSummary{
						Pass:  2,
						Fail:  0,
						Skip:  0,
						Error: 0,
						Warn:  0,
					},
				},
			},
		},
		TestCase{
			PolicyPaths:   []string{"../../../samples/best_practices/require_pod_requests_limits.yaml"},
			ResourcePaths: []string{"../../../test/resources/pod_with_latest_tag.yaml"},
			expectedPolicyReports: []preport.PolicyReport{
				preport.PolicyReport{
					Summary: preport.PolicyReportSummary{
						Pass:  0,
						Fail:  1,
						Skip:  0,
						Error: 0,
						Warn:  0,
					},
				},
			},
		},
	}

	for _, tt := range testcases {
		validateEngineResponses, _, _, skippedPolicies, _ := applyCommandHelper(tt.ResourcePaths, false, true, "", "", "", "", tt.PolicyPaths)
		resps := buildPolicyReports(validateEngineResponses, skippedPolicies)
		for i, resp := range resps {
			thisSummary := tt.expectedPolicyReports[i].Summary
			thisRespSummary := resp.UnstructuredContent()["summary"].(map[string]interface{})
			assert.Assert(t, thisRespSummary[preport.StatusPass].(int64) == int64(thisSummary.Pass))
			assert.Assert(t, thisRespSummary[preport.StatusFail].(int64) == int64(thisSummary.Fail))
			assert.Assert(t, thisRespSummary[preport.StatusSkip].(int64) == int64(thisSummary.Skip))
			assert.Assert(t, thisRespSummary[preport.StatusWarn].(int64) == int64(thisSummary.Warn))
			assert.Assert(t, thisRespSummary[preport.StatusError].(int64) == int64(thisSummary.Error))
		}
	}
}
