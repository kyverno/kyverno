package apply

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/report"
	"github.com/stretchr/testify/assert"
	openreportsv1alpha1 "openreports.io/apis/openreports.io/v1alpha1"
)

func Test_Apply(t *testing.T) {
	type TestCase struct {
		expectedReports []openreportsv1alpha1.Report
		config          ApplyCommandConfig
		stdinFile       string
	}
	// copy disallow_latest_tag.yaml to local path
	localFileName, err := copyFileToThisDir("../../../../../test/best_practices/disallow_latest_tag.yaml")
	assert.NoError(t, err)
	defer func() { _ = os.Remove(localFileName) }()

	testcases := []*TestCase{
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/best_practices/disallow_latest_tag.yaml"},
				ResourcePaths: []string{"../../../../../test/resources/pod_with_version_tag.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  2,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{localFileName},
				ResourcePaths: []string{"../../../../../test/resources/pod_with_version_tag.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  2,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/best_practices/disallow_latest_tag.yaml"},
				ResourcePaths: []string{"../../../../../test/resources/pod_with_latest_tag.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/apply/policies"},
				ResourcePaths: []string{"../../../../../test/cli/apply/resource"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  2,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/best_practices/disallow_latest_tag.yaml"},
				ResourcePaths: []string{"../../../../../test/resources/pod_with_latest_tag.yaml"},
				PolicyReport:  true,
				AuditWarn:     true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  1,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"-"},
				ResourcePaths: []string{"../../../../../test/resources/pod_with_latest_tag.yaml"},
				PolicyReport:  true,
				AuditWarn:     true,
				warnExitCode:  3,
			},
			stdinFile: "../../../../../test/best_practices/disallow_latest_tag.yaml",
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  1,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/best_practices/disallow_latest_tag.yaml"},
				ResourcePaths: []string{"-"},
				PolicyReport:  true,
				AuditWarn:     true,
			},
			stdinFile: "../../../../../test/resources/pod_with_latest_tag.yaml",
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  1,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/apply/policies-set"},
				ResourcePaths: []string{"../../../../../test/cli/apply/resources-set"},
				Variables:     []string{"request.operation=UPDATE"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  2,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-admission-policy/check-deployments-replica/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-admission-policy/check-deployments-replica/deployment1.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-admission-policy/check-deployments-replica/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-admission-policy/check-deployments-replica/deployment2.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  0,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-admission-policy/disallow-host-path/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-admission-policy/disallow-host-path/pod1.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-admission-policy/disallow-host-path/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-admission-policy/disallow-host-path/pod2.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  0,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-admission-policy/check-deployment-labels/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-admission-policy/check-deployment-labels/deployment1.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-admission-policy/check-deployment-labels/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-admission-policy/check-deployment-labels/deployment2.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  0,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths: []string{"../../../../../test/cli/test-validating-admission-policy/with-bindings-1/policy.yaml"},
				ResourcePaths: []string{
					"../../../../../test/cli/test-validating-admission-policy/with-bindings-1/deployment1.yaml",
					"../../../../../test/cli/test-validating-admission-policy/with-bindings-1/deployment2.yaml",
				},
				PolicyReport: true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  0,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths: []string{"../../../../../test/cli/test-validating-admission-policy/with-bindings-2/policy.yaml"},
				ResourcePaths: []string{
					"../../../../../test/cli/test-validating-admission-policy/with-bindings-2/deployment1.yaml",
					"../../../../../test/cli/test-validating-admission-policy/with-bindings-2/deployment2.yaml",
				},
				PolicyReport: true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  0,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths: []string{"../../../../../test/cli/test-validating-admission-policy/with-bindings-3/policy.yaml"},
				ResourcePaths: []string{
					"../../../../../test/cli/test-validating-admission-policy/with-bindings-3/deployment1.yaml",
					"../../../../../test/cli/test-validating-admission-policy/with-bindings-3/deployment2.yaml",
					"../../../../../test/cli/test-validating-admission-policy/with-bindings-3/deployment3.yaml",
				},
				ValuesFile:   "../../../../../test/cli/test-validating-admission-policy/with-bindings-3/values.yaml",
				PolicyReport: true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  2,
					Fail:  2,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths: []string{"../../../../../test/cli/test-validating-admission-policy/with-bindings-4/policy.yaml"},
				ResourcePaths: []string{
					"../../../../../test/cli/test-validating-admission-policy/with-bindings-4/deployment1.yaml",
					"../../../../../test/cli/test-validating-admission-policy/with-bindings-4/deployment2.yaml",
				},
				PolicyReport: true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"https://github.com/kyverno/policies/best-practices/require-labels/", "../../../../../test/best_practices/disallow_latest_tag.yaml"},
				ResourcePaths: []string{"../../../../../test/resources/pod_with_version_tag.yaml"},
				GitBranch:     "main",
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  2,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			// Same as the above test case but the policy paths are reordered
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/best_practices/disallow_latest_tag.yaml", "https://github.com/kyverno/policies/best-practices/require-labels/"},
				ResourcePaths: []string{"../../../../../test/resources/pod_with_version_tag.yaml"},
				GitBranch:     "main",
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  2,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths: []string{
					"../../../../../test/cli/apply/type/policy1.yaml",
					"../../../../../test/cli/apply/type/policy2.yaml",
					"../../../../../test/cli/apply/type/policy3.yaml",
				},
				ResourcePaths: []string{"../../../../../test/cli/apply/type/resource.yaml"},
				GitBranch:     "main",
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  3,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
	}

	compareSummary := func(expected openreportsv1alpha1.ReportSummary, actual openreportsv1alpha1.ReportSummary, desc string) {
		assert.Equal(t, actual.Pass, expected.Pass, desc)
		assert.Equal(t, actual.Fail, expected.Fail, desc)
		assert.Equal(t, actual.Skip, expected.Skip, desc)
		assert.Equal(t, actual.Warn, expected.Warn, desc)
		assert.Equal(t, actual.Error, expected.Error, desc)
		assert.Equal(t, actual.Pass, expected.Pass, desc)

	}

	verifyTestcase := func(t *testing.T, tc *TestCase, compareSummary func(openreportsv1alpha1.ReportSummary, openreportsv1alpha1.ReportSummary, string)) {
		if tc.stdinFile != "" {
			oldStdin := os.Stdin
			input, err := os.OpenFile(tc.stdinFile, os.O_RDONLY, 0)
			assert.NoError(t, err)
			os.Stdin = input
			defer func() {
				// Restore original Stdin
				os.Stdin = oldStdin
				_ = input.Close()
			}()
		}
		desc := fmt.Sprintf("Policies: [%s], / Resources: [%s]", strings.Join(tc.config.PolicyPaths, ","), strings.Join(tc.config.ResourcePaths, ","))

		_, _, _, responses, err := tc.config.applyCommandHelper(os.Stdout)
		assert.NoError(t, err, desc)

		clustered, _ := report.ComputePolicyReports(tc.config.AuditWarn, responses...)
		assert.Greater(t, len(clustered), 0, "policy reports should not be empty: %s", desc)
		combined := []openreportsv1alpha1.ClusterReport{
			report.MergeClusterReports(clustered),
		}
		assert.Equal(t, len(combined), len(tc.expectedReports))
		for i, resp := range combined {
			compareSummary(tc.expectedReports[i].Summary, resp.Summary, desc)
		}
	}

	for _, tc := range testcases {
		t.Run("", func(t *testing.T) {
			verifyTestcase(t, tc, compareSummary)
		})
	}
}

type TestCase struct {
	expectedReports []openreportsv1alpha1.Report
	config          ApplyCommandConfig
	stdinFile       string
}

func Test_Apply_ValidatingPolicies(t *testing.T) {
	testcases := []*TestCase{
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-policy/check-deployment-labels/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-policy/check-deployment-labels/deployment1.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-policy/check-deployment-labels/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-policy/check-deployment-labels/deployment2.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  0,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-policy/check-deployments-replica/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-policy/check-deployments-replica/deployment1.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-policy/check-deployments-replica/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-policy/check-deployments-replica/deployment2.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  0,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-policy/disallow-host-path/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-policy/disallow-host-path/pod1.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-policy/disallow-host-path/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-policy/disallow-host-path/pod2.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  0,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:  []string{"../../../../../test/cli/test-validating-policy/json-check-dockerfile/policy.yaml"},
				JSONPaths:    []string{"../../../../../test/cli/test-validating-policy/json-check-dockerfile/payload.json"},
				PolicyReport: true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-cel-exceptions/check-deployment-labels/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-cel-exceptions/check-deployment-labels/skipped-deployment.yaml"},
				Exception:     []string{"../../../../../test/cli/test-cel-exceptions/check-deployment-labels/exception.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  0,
					Fail:  0,
					Skip:  1,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-cel-exceptions/check-deployment-labels/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-cel-exceptions/check-deployment-labels/bad-deployment.yaml"},
				Exception:     []string{"../../../../../test/cli/test-cel-exceptions/check-deployment-labels/exception.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  0,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-cel-exceptions/check-deployment-labels/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-cel-exceptions/check-deployment-labels/good-deployment.yaml"},
				Exception:     []string{"../../../../../test/cli/test-cel-exceptions/check-deployment-labels/exception.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-policy/policy-with-cm/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-policy/policy-with-cm/pod1.yaml"},
				ContextPath:   "../../../../../test/cli/test-validating-policy/policy-with-cm/context.yaml",
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-policy/policy-with-cm/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-policy/policy-with-cm/pod2.yaml"},
				ContextPath:   "../../../../../test/cli/test-validating-policy/policy-with-cm/context.yaml",
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  0,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths: []string{"../../../../../test/cli/test-validating-policy/json-check-variables/policy.yaml"},
				JSONPaths:   []string{"../../../../../test/cli/test-validating-policy/json-check-variables/payload.json"},

				PolicyReport: true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  0,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
	}

	for _, tc := range testcases {
		t.Run("", func(t *testing.T) {
			verifyTestcase(t, tc, compareSummary)
		})
	}
}

func Test_Apply_ImageVerificationPolicies(t *testing.T) {
	testcases := []*TestCase{
		{
			config: ApplyCommandConfig{
				PolicyPaths: []string{"../../../../../test/conformance/chainsaw/image-validating-policies/match-conditions/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/conformance/chainsaw/image-validating-policies/match-conditions/good-pod.yaml",
					"../../../../../test/conformance/chainsaw/image-validating-policies/match-conditions/bad-pod.yaml"},
				PolicyReport: true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths: []string{"../../../../../test/cli/test-image-validating-policy/check-json/ivpol-json.yaml"},
				JSONPaths: []string{"../../../../../test/cli/test-image-validating-policy/check-json/ivpol-payload-pass.json",
					"../../../../../test/cli/test-image-validating-policy/check-json/ivpol-payload-fail.json"},
				PolicyReport: true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-image-validating-policy/with-cel-exceptions/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-image-validating-policy/with-cel-exceptions/resources.yaml"},
				Exception:     []string{"../../../../../test/cli/test-image-validating-policy/with-cel-exceptions/exception.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  1,
					Skip:  1,
					Error: 0,
					Warn:  0,
				},
			}},
		},
	}

	for _, tc := range testcases {
		t.Run("", func(t *testing.T) {
			verifyTestcase(t, tc, compareSummary)
		})
	}
}

func Test_Apply_DeletingPolicies(t *testing.T) {
	testcases := []*TestCase{
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-deleting-policy/deleting-pod-by-name/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-deleting-policy/deleting-pod-by-name/resource.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:  []string{"../../../../../test/cli/test-deleting-policy/deleting-json/policy.yaml"},
				JSONPaths:    []string{"../../../../../test/cli/test-deleting-policy/deleting-json/payload.json"},
				PolicyReport: true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  1,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
	}

	for _, tc := range testcases {
		t.Run("", func(t *testing.T) {
			verifyTestcase(t, tc, compareSummary)
		})
	}
}

func Test_Apply_MutatingAdmissionPolicies(t *testing.T) {
	testcases := []*TestCase{
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-mutating-admission-policy/with-match-conditions/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-mutating-admission-policy/with-match-conditions/resource.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  0,
					Skip:  1,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-mutating-admission-policy/with-binding-object-selector/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-mutating-admission-policy/with-binding-object-selector/resource.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-mutating-admission-policy/with-binding-namespace-selector/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-mutating-admission-policy/with-binding-namespace-selector/resource.yaml"},
				ValuesFile:    "../../../../../test/cli/test-mutating-admission-policy/with-binding-namespace-selector/values.yaml",
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  2,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-mutating-admission-policy/with-binding-exclude-resources/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-mutating-admission-policy/with-binding-exclude-resources/resource.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-mutating-admission-policy/with-binding-match-resources/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-mutating-admission-policy/with-binding-match-resources/resource.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-mutating-admission-policy/specify-object-selector/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-mutating-admission-policy/specify-object-selector/resource.yaml"},
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  1,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-mutating-admission-policy/specify-namespace-selector/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-mutating-admission-policy/specify-namespace-selector/resource.yaml"},
				ValuesFile:    "../../../../../test/cli/test-mutating-admission-policy/specify-namespace-selector/values.yaml",
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  2,
					Fail:  0,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
	}

	for _, tc := range testcases {
		t.Run("", func(t *testing.T) {
			verifyTestcase(t, tc, compareSummary)
		})
	}
}

func compareSummary(t *testing.T, expected openreportsv1alpha1.ReportSummary, actual openreportsv1alpha1.ReportSummary, desc string) {
	assert.Equal(t, actual.Pass, expected.Pass, desc)
	assert.Equal(t, actual.Fail, expected.Fail, desc)
	assert.Equal(t, actual.Skip, expected.Skip, desc)
	assert.Equal(t, actual.Warn, expected.Warn, desc)
	assert.Equal(t, actual.Error, expected.Error, desc)
}

func verifyTestcase(t *testing.T, tc *TestCase, compareSummary func(*testing.T, openreportsv1alpha1.ReportSummary, openreportsv1alpha1.ReportSummary, string)) {
	if tc.stdinFile != "" {
		oldStdin := os.Stdin
		input, err := os.OpenFile(tc.stdinFile, os.O_RDONLY, 0)
		assert.NoError(t, err)
		os.Stdin = input
		defer func() {
			os.Stdin = oldStdin
			_ = input.Close()
		}()
	}
	desc := fmt.Sprintf("Policies: [%s], / Resources: [%s], JSON payload: [%s]",
		strings.Join(tc.config.PolicyPaths, ","),
		strings.Join(tc.config.ResourcePaths, ","),
		strings.Join(tc.config.JSONPaths, ","),
	)

	_, _, _, responses, err := tc.config.applyCommandHelper(os.Stdout)
	assert.NoError(t, err, desc)

	clustered, _ := report.ComputePolicyReports(tc.config.AuditWarn, responses...)
	assert.Greater(t, len(clustered), 0, "policy reports should not be empty: %s", desc)
	combined := []openreportsv1alpha1.ClusterReport{
		report.MergeClusterReports(clustered),
	}

	assert.Equal(t, len(combined), len(tc.expectedReports), "Number of combined reports does not match expected: "+desc)
	for i, resp := range combined {
		compareSummary(t, tc.expectedReports[i].Summary, resp.Summary, desc)
	}
}

func copyFileToThisDir(sourceFile string) (string, error) {
	input, err := os.ReadFile(sourceFile)
	if err != nil {
		return "", err
	}

	return filepath.Base(sourceFile), os.WriteFile(filepath.Base(sourceFile), input, 0o644)
}

func TestCommand(t *testing.T) {
	cmd := Command()
	cmd.SetArgs([]string{
		"../../_testdata/apply/test-1/policy.yaml",
		"--resource",
		"../../_testdata/apply/test-1/resources.yaml",
	})
	err := cmd.Execute()
	assert.NoError(t, err)
}

func TestCommandWithInvalidArg(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	err := cmd.Execute()
	assert.Error(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	expected := `Error: require policy`
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(out)))
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

func TestCommandWithJsonAndResource(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	cmd.SetArgs([]string{"--json", "foo", "--resource", "bar", "policy"})
	err := cmd.Execute()
	assert.Error(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	expected := `Error: both resource and json files can not be used together, use one or the other`
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(out)))
}

func TestCommandWarnExitCode(t *testing.T) {
	var warnExitCode = 3

	cmd := Command()
	cmd.SetArgs([]string{
		"../../_testdata/apply/test-2/policy.yaml",
		"--resource",
		"../../_testdata/apply/test-2/resources.yaml",
		"--audit-warn",
		"--warn-exit-code",
		strconv.Itoa(warnExitCode),
	})
	err := cmd.Execute()
	if err != nil {
		switch e := err.(type) {
		case WarnExitCodeError:
			assert.Equal(t, warnExitCode, e.ExitCode)
		default:
			assert.Fail(t, "Expecting WarnExitCodeError")
		}
	}
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
