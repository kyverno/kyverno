package apply

import (
	"testing"

	preport "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"gotest.tools/assert"
)

func Test_Apply(t *testing.T) {
	type TestCase struct {
		PolicyPaths           []string
		ResourcePaths         []string
		expectedPolicyReports []preport.PolicyReport
	}

	testcases := []TestCase{
		{
			PolicyPaths:   []string{"../../../../test/best_practices/disallow_latest_tag.yaml"},
			ResourcePaths: []string{"../../../../test/resources/pod_with_version_tag.yaml"},
			expectedPolicyReports: []preport.PolicyReport{
				{
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
		{
			PolicyPaths:   []string{"../../../../test/best_practices/disallow_latest_tag.yaml"},
			ResourcePaths: []string{"../../../../test/resources/pod_with_latest_tag.yaml"},
			expectedPolicyReports: []preport.PolicyReport{
				{
					Summary: preport.PolicyReportSummary{
						Pass:  1,
						Fail:  1,
						Skip:  0,
						Error: 0,
						Warn:  0,
					},
				},
			},
		},
		{
			PolicyPaths:   []string{"../../../../test/cli/apply/policies"},
			ResourcePaths: []string{"../../../../test/cli/apply/resource"},
			expectedPolicyReports: []preport.PolicyReport{
				{
					Summary: preport.PolicyReportSummary{
						Pass:  1,
						Fail:  1,
						Skip:  8,
						Error: 0,
						Warn:  2,
					},
				},
			},
		},
	}

	compareSummary := func(expected preport.PolicyReportSummary, actual map[string]interface{}) {
		assert.Equal(t, actual[preport.StatusPass].(int64), int64(expected.Pass))
		assert.Equal(t, actual[preport.StatusFail].(int64), int64(expected.Fail))
		assert.Equal(t, actual[preport.StatusSkip].(int64), int64(expected.Skip))
		assert.Equal(t, actual[preport.StatusWarn].(int64), int64(expected.Warn))
		assert.Equal(t, actual[preport.StatusError].(int64), int64(expected.Error))
	}

	for _, tc := range testcases {
		_, _, _, info, _ := applyCommandHelper(tc.ResourcePaths, "", false, true, "", "", "", "", tc.PolicyPaths, false, false)
		resps := buildPolicyReports(info)
		for i, resp := range resps {
			compareSummary(tc.expectedPolicyReports[i].Summary, resp.UnstructuredContent()["summary"].(map[string]interface{}))
		}
	}
}
