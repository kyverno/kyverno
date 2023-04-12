package apply

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	preport "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"gotest.tools/assert"
)

func Test_Apply(t *testing.T) {
	type TestCase struct {
		gitBranch             string
		expectedPolicyReports []preport.PolicyReport
		config                ApplyCommandConfig
		stdinFile             string
	}
	// copy disallow_latest_tag.yaml to local path
	localFileName, err := copyFileToThisDir("../../../../test/best_practices/disallow_latest_tag.yaml")
	assert.NilError(t, err)
	defer func() { _ = os.Remove(localFileName) }()

	testcases := []*TestCase{
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../test/best_practices/disallow_latest_tag.yaml"},
				ResourcePaths: []string{"../../../../test/resources/pod_with_version_tag.yaml"},
				PolicyReport:  true,
			},
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
			config: ApplyCommandConfig{
				PolicyPaths:   []string{localFileName},
				ResourcePaths: []string{"../../../../test/resources/pod_with_version_tag.yaml"},
				PolicyReport:  true,
			},
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
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../test/best_practices/disallow_latest_tag.yaml"},
				ResourcePaths: []string{"../../../../test/resources/pod_with_latest_tag.yaml"},
				PolicyReport:  true,
			},
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
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../test/cli/apply/policies"},
				ResourcePaths: []string{"../../../../test/cli/apply/resource"},
				PolicyReport:  true,
			},
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
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../test/best_practices/disallow_latest_tag.yaml"},
				ResourcePaths: []string{"../../../../test/resources/pod_with_latest_tag.yaml"},
				PolicyReport:  true,
				AuditWarn:     true,
			},
			expectedPolicyReports: []preport.PolicyReport{
				{
					Summary: preport.PolicyReportSummary{
						Pass:  1,
						Fail:  0,
						Skip:  0,
						Error: 0,
						Warn:  1,
					},
				},
			},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"-"},
				ResourcePaths: []string{"../../../../test/resources/pod_with_latest_tag.yaml"},
				PolicyReport:  true,
				AuditWarn:     true,
			},
			stdinFile: "../../../../test/best_practices/disallow_latest_tag.yaml",
			expectedPolicyReports: []preport.PolicyReport{
				{
					Summary: preport.PolicyReportSummary{
						Pass:  1,
						Fail:  0,
						Skip:  0,
						Error: 0,
						Warn:  1,
					},
				},
			},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../test/best_practices/disallow_latest_tag.yaml"},
				ResourcePaths: []string{"-"},
				PolicyReport:  true,
				AuditWarn:     true,
			},
			stdinFile: "../../../../test/resources/pod_with_latest_tag.yaml",
			expectedPolicyReports: []preport.PolicyReport{
				{
					Summary: preport.PolicyReportSummary{
						Pass:  1,
						Fail:  0,
						Skip:  0,
						Error: 0,
						Warn:  1,
					},
				},
			},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"https://github.com/kyverno/policies/openshift/team-validate-ns-name/"},
				ResourcePaths: []string{"../../../../test/openshift/team-validate-ns-name.yaml"},
				GitBranch:     "main",
				PolicyReport:  true,
			},
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
	}

	compareSummary := func(expected preport.PolicyReportSummary, actual map[string]interface{}, desc string) {
		assert.Equal(t, actual[preport.StatusPass].(int64), int64(expected.Pass), desc)
		assert.Equal(t, actual[preport.StatusFail].(int64), int64(expected.Fail), desc)
		assert.Equal(t, actual[preport.StatusSkip].(int64), int64(expected.Skip), desc)
		assert.Equal(t, actual[preport.StatusWarn].(int64), int64(expected.Warn), desc)
		assert.Equal(t, actual[preport.StatusError].(int64), int64(expected.Error), desc)
	}

	verifyTestcase := func(t *testing.T, tc *TestCase, compareSummary func(preport.PolicyReportSummary, map[string]interface{}, string)) {
		if tc.stdinFile != "" {
			oldStdin := os.Stdin
			input, err := os.OpenFile(tc.stdinFile, os.O_RDONLY, 0)
			assert.NilError(t, err)
			os.Stdin = input
			defer func() {
				// Restore original Stdin
				os.Stdin = oldStdin
				_ = input.Close()
			}()
		}
		desc := fmt.Sprintf("Policies: [%s], / Resources: [%s]", strings.Join(tc.config.PolicyPaths, ","), strings.Join(tc.config.ResourcePaths, ","))
		// prevent os.Exit from being called
		osExit = func(code int) {
			assert.Check(t, false, "os.Exit(%d) should not be called: %s", code, desc)
		}

		defer func() { osExit = os.Exit }()
		_, _, _, info, err := tc.config.applyCommandHelper()
		assert.NilError(t, err, desc)

		resps := buildPolicyReports(tc.config.AuditWarn, info...)
		assert.Assert(t, len(resps) > 0, "policy reports should not be empty: %s", desc)
		for i, resp := range resps {
			compareSummary(tc.expectedPolicyReports[i].Summary, resp.UnstructuredContent()["summary"].(map[string]interface{}), desc)
		}
	}

	for _, tc := range testcases {
		verifyTestcase(t, tc, compareSummary)
	}
}

func copyFileToThisDir(sourceFile string) (string, error) {
	input, err := os.ReadFile(sourceFile)
	if err != nil {
		return "", err
	}

	return filepath.Base(sourceFile), os.WriteFile(filepath.Base(sourceFile), input, 0644)
}
