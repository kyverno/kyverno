package apply

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/report"
	pkgdeprecations "github.com/kyverno/kyverno/pkg/deprecations"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestMain(m *testing.M) {
	log.SetLogger(logr.Discard())
	m.Run()
}

func TestPrintSkippedAndInvalidPolicies(t *testing.T) {
	var out bytes.Buffer
	printSkippedAndInvalidPolicies(&out, SkippedInvalidPolicies{
		skipped: []PolicyDiagnostic{{
			name:   "skipped-policy",
			reason: "missing variable `request.operation`",
		}},
		invalid: []PolicyDiagnostic{{
			name:   "invalid-policy",
			reason: "failed to compile policy invalid-policy (spec.matchConditions[0].expression: Forbidden: variables cannot be referenced)",
		}},
	})

	printed := out.String()
	assert.Contains(t, printed, "Policies Skipped:")
	assert.Contains(t, printed, "1. skipped-policy: missing variable `request.operation`")
	assert.Contains(t, printed, "Invalid Policies:")
	assert.Contains(t, printed, "1. invalid-policy: failed to compile policy invalid-policy")
	assert.Contains(t, printed, "variables cannot be referenced")
}

func Test_Apply(t *testing.T) {
	type TestCase struct {
		expectedReports []openreportsv1alpha1.Report
		config          ApplyCommandConfig
		stdinFile       string
	}
	testcases := []*TestCase{
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
				PolicyPaths: []string{"../../../../../test/cli/test-validating-admission-policy/check-user-info/policy.yaml"},
				ResourcePaths: []string{
					"../../../../../test/cli/test-validating-admission-policy/check-user-info/deployment.yaml",
				},
				UserInfoPath: "../../../../../test/cli/test-validating-admission-policy/check-user-info/userinfo.yaml",
				PolicyReport: true,
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
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-policy/exceptions-check-deployment-labels/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-policy/exceptions-check-deployment-labels/skipped-deployment.yaml"},
				Exception:     []string{"../../../../../test/cli/test-validating-policy/exceptions-check-deployment-labels/exception.yaml"},
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
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-policy/exceptions-check-deployment-labels/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-policy/exceptions-check-deployment-labels/bad-deployment.yaml"},
				Exception:     []string{"../../../../../test/cli/test-validating-policy/exceptions-check-deployment-labels/exception.yaml"},
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
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-policy/exceptions-check-deployment-labels/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-policy/exceptions-check-deployment-labels/good-deployment.yaml"},
				Exception:     []string{"../../../../../test/cli/test-validating-policy/exceptions-check-deployment-labels/exception.yaml"},
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
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-policy/restrict-image-registries/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-policy/restrict-image-registries/resource.yaml"},
				ContextPath:   "../../../../../test/cli/test-validating-policy/restrict-image-registries/context.yaml",
				ValuesFile:    "../../../../../test/cli/test-validating-policy/restrict-image-registries/value.yaml",
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  3,
					Fail:  4,
					Skip:  0,
					Error: 0,
					Warn:  0,
				},
			}},
		},
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-policy/allowed-base-images/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-policy/allowed-base-images/resource.yaml"},
				ContextPath:   "../../../../../test/cli/test-validating-policy/allowed-base-images/context.yaml",
				PolicyReport:  true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass:  2,
					Fail:  3,
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
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-policy/empty-message/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-policy/empty-message/pod-fail.yaml"},
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
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-policy/empty-message/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-policy/empty-message/pod-pass.yaml"},
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
		// JSON-mode policy applied to a K8s resource should evaluate via JSON path
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-validating-policy/json-mode-on-resource/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-validating-policy/check-deployment-labels/deployment1.yaml"},
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
	}

	for _, tc := range testcases {
		t.Run("", func(t *testing.T) {
			verifyTestcase(t, tc, compareSummary)
		})
	}
}

// Test_Apply_JsonPayload_K8sMode_NoSegfault verifies that applying a
// Kubernetes-mode policy against a JSON payload does not panic (segfault).
// The K8s-mode policy should be gracefully skipped with zero results.
func Test_Apply_JsonPayload_K8sMode_NoSegfault(t *testing.T) {
	config := ApplyCommandConfig{
		PolicyPaths:  []string{"../../../../../test/cli/test-validating-policy/json-payload-k8s-mode-policy/policy.yaml"},
		JSONPaths:    []string{"../../../../../test/cli/test-validating-policy/json-payload-k8s-mode-policy/payload.json"},
		PolicyReport: true,
	}
	_, _, _, responses, err := config.applyCommandHelper(io.Discard)
	assert.NoError(t, err, "should not crash with segfault")
	// K8s-mode policy should be skipped for JSON payloads, so no responses expected
	assert.Equal(t, 0, len(responses), "K8s-mode policies should be skipped for JSON payloads")
}

func Test_Apply_ImageVerificationPolicies(t *testing.T) {
	testcases := []*TestCase{
		{
			config: ApplyCommandConfig{
				PolicyPaths: []string{"../../../../../test/conformance/chainsaw/image-validating-policies/match-conditions/policy.yaml"},
				ResourcePaths: []string{
					"../../../../../test/conformance/chainsaw/image-validating-policies/match-conditions/good-pod.yaml",
					"../../../../../test/conformance/chainsaw/image-validating-policies/match-conditions/bad-pod.yaml",
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
				PolicyPaths: []string{"../../../../../test/cli/test-image-validating-policy/check-json/ivpol-json.yaml"},
				JSONPaths: []string{
					"../../../../../test/cli/test-image-validating-policy/check-json/ivpol-payload-pass.json",
					"../../../../../test/cli/test-image-validating-policy/check-json/ivpol-payload-fail.json",
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
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-image-validating-policy/empty-message/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-image-validating-policy/empty-message/bad-pod.yaml"},
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
				PolicyPaths:   []string{"../../../../../test/cli/test-image-validating-policy/empty-message/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-image-validating-policy/empty-message/good-pod.yaml"},
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
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../../../../test/cli/test-deleting-policy/deleting-pod-by-namespaceObject/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-deleting-policy/deleting-pod-by-namespaceObject/resource.yaml"},
				ValuesFile:    "../../../../../test/cli/test-deleting-policy/deleting-pod-by-namespaceObject/values.yaml",
				PolicyReport:  true,
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
				PolicyPaths:   []string{"../../../../../test/cli/test-deleting-policy/use-resource-lib-pass/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-deleting-policy/use-resource-lib-pass/resource.yaml"},
				ContextPath:   "../../../../../test/cli/test-deleting-policy/use-resource-lib-pass/context.yaml",
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
				PolicyPaths:   []string{"../../../../../test/cli/test-deleting-policy/use-resource-lib-fail/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-deleting-policy/use-resource-lib-fail/resource.yaml"},
				ContextPath:   "../../../../../test/cli/test-deleting-policy/use-resource-lib-fail/context.yaml",
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
	desc := fmt.Sprintf(
		"Policies: [%s], / Resources: [%s], JSON payload: [%s]",
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

func TestApplyBlocksLegacyClusterPolicy(t *testing.T) {
	blocked := ApplyCommandConfig{
		PolicyPaths:   []string{"../../../../../test/cli/test-legacy-policies/legacy-clusterpolicy/policy.yaml"},
		ResourcePaths: []string{"../../../../../test/cli/test-legacy-policies/legacy-clusterpolicy/resources.yaml"},
		PolicyReport:  true,
	}
	_, _, _, _, err := blocked.applyCommandHelper(io.Discard)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v1 ClusterPolicy is no longer accepted")
	assert.Contains(t, err.Error(), pkgdeprecations.MigrationGuideURL)
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

func Test_ValidatingPolicy_DefaultMessage(t *testing.T) {
	config := ApplyCommandConfig{
		PolicyPaths:   []string{"../../../../../test/cli/test-validating-policy/empty-message/policy.yaml"},
		ResourcePaths: []string{"../../../../../test/cli/test-validating-policy/empty-message/pod-fail.yaml"},
		PolicyReport:  true,
	}

	_, _, _, responses, err := config.applyCommandHelper(os.Stdout)
	assert.NoError(t, err)

	// Check the responses for the correct message
	found := false
	var actualMessage string
	for _, response := range responses {
		for _, rule := range response.PolicyResponse.Rules {
			if rule.Status() == engineapi.RuleStatusFail {
				found = true
				actualMessage = rule.Message()
				assert.Contains(t, actualMessage, "CEL expression validation failed at index",
					"ValidatingPolicy should show default message when message field is empty")
				assert.NotEmpty(t, actualMessage, "Message should not be empty")
			}
		}
	}
	assert.True(t, found, "Should have at least one failed rule")
}

func Test_ImageValidatingPolicy_DefaultMessage(t *testing.T) {
	config := ApplyCommandConfig{
		PolicyPaths:   []string{"../../../../../test/cli/test-image-validating-policy/empty-message/policy.yaml"},
		ResourcePaths: []string{"../../../../../test/cli/test-image-validating-policy/empty-message/bad-pod.yaml"},
		PolicyReport:  true,
	}

	_, _, _, responses, err := config.applyCommandHelper(os.Stdout)
	assert.NoError(t, err)

	// Check the responses for the correct message
	found := false
	var actualMessage string
	for _, response := range responses {
		for _, rule := range response.PolicyResponse.Rules {
			if rule.Status() == engineapi.RuleStatusFail {
				found = true
				actualMessage = rule.Message()
				assert.Contains(t, actualMessage, "CEL expression validation failed at index",
					"ImageValidatingPolicy should show default message when message field is empty")
				assert.NotEmpty(t, actualMessage, "Message should not be empty")
			}
		}
	}
	assert.True(t, found, "Should have at least one failed rule")
}

func Test_Apply_PoliciesWithCRD(t *testing.T) {
	testcases := []*TestCase{
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../_testdata/apply/test-3/resource-validating-policy/policy.yml"},
				ResourcePaths: []string{"../../_testdata/apply/test-3/resources/resource.yml"},
				CrdPaths:      []string{"../../_testdata/apply/test-3/crd/crd.yml"},
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
				PolicyPaths:   []string{"../../../../../test/cli/test-mutating-policy/mutate-custom-crd/policy.yaml"},
				ResourcePaths: []string{"../../../../../test/cli/test-mutating-policy/mutate-custom-crd/widget.yaml"},
				CrdPaths:      []string{"../../../../../test/cli/test-mutating-policy/mutate-custom-crd/crds/widget-crd.yaml"},
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
	}

	for _, tc := range testcases {
		t.Run("", func(t *testing.T) {
			verifyTestcase(t, tc, compareSummary)
		})
	}
}

func Test_Apply_ValidatingPoliciesWithMultipleCRDS(t *testing.T) {
	testcases := []*TestCase{
		{
			config: ApplyCommandConfig{
				PolicyPaths:   []string{"../../_testdata/apply/test-4/resource-validating-policy/policy.yml"},
				ResourcePaths: []string{"../../_testdata/apply/test-4/resources/foo.yml", "../../_testdata/apply/test-4/resources/bar.yml"},
				CrdPaths:      []string{"../../_testdata/apply/test-4/crd/crds.yml"},
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

func TestCommandCRDKubeEnable(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	cmd.SetArgs([]string{
		"../../_testdata/apply/test-2/policy.yaml",
		"--resource",
		"../../_testdata/apply/test-2/resources.yaml",
		"--crd-paths",
		"./crd.yml",
		"--kubeconfig",
		"./kubeconfig.yaml",
	})
	err := cmd.Execute()
	assert.Error(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	expected := `Error: crd-paths and kubeconfig flags are mutually exclusive, please use only one of them`
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(out)))
}

func Test_Apply_AuthzPolicies(t *testing.T) {
	testcases := []*TestCase{
		// HTTP allow
		{
			config: ApplyCommandConfig{
				PolicyPaths:      []string{"../../../../../test/cli/test-validating-policy/http-allow/policy.yaml"},
				HTTPPayloadPaths: []string{"../../../../../test/cli/test-validating-policy/http-allow/request.json"},
				PolicyReport:     true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass: 1,
				},
			}},
		},
		// HTTP deny
		{
			config: ApplyCommandConfig{
				PolicyPaths:      []string{"../../../../../test/cli/test-validating-policy/http-deny/policy.yaml"},
				HTTPPayloadPaths: []string{"../../../../../test/cli/test-validating-policy/http-deny/request.json"},
				PolicyReport:     true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Fail: 1,
				},
			}},
		},
		// Envoy allow
		{
			config: ApplyCommandConfig{
				PolicyPaths:       []string{"../../../../../test/cli/test-validating-policy/envoy-allow/policy.yaml"},
				EnvoyPayloadPaths: []string{"../../../../../test/cli/test-validating-policy/envoy-allow/request.json"},
				PolicyReport:      true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass: 1,
				},
			}},
		},
		// Envoy deny
		{
			config: ApplyCommandConfig{
				PolicyPaths:       []string{"../../../../../test/cli/test-validating-policy/envoy-deny/policy.yaml"},
				EnvoyPayloadPaths: []string{"../../../../../test/cli/test-validating-policy/envoy-deny/request.json"},
				PolicyReport:      true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Fail: 1,
				},
			}},
		},
		// Envoy JWT (3 requests: 2 denied, 1 allowed)
		{
			config: ApplyCommandConfig{
				PolicyPaths: []string{"../../../../../test/cli/test-validating-policy/envoy-jwt/policy.yaml"},
				EnvoyPayloadPaths: []string{
					"../../../../../test/cli/test-validating-policy/envoy-jwt/request-empty.json",
					"../../../../../test/cli/test-validating-policy/envoy-jwt/request-forbidden.json",
					"../../../../../test/cli/test-validating-policy/envoy-jwt/request-pass.json",
				},
				PolicyReport: true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass: 1,
					Fail: 2,
				},
			}},
		},
	}
	for i, tc := range testcases {
		t.Run(fmt.Sprintf("authz-case-%d", i), func(t *testing.T) {
			verifyTestcase(t, tc, compareSummary)
		})
	}
}

func Test_Apply_AuthzPolicies_MixedHTTPAndEnvoy(t *testing.T) {
	testcases := []*TestCase{
		{
			config: ApplyCommandConfig{
				PolicyPaths: []string{
					"../../../../../test/cli/test-validating-policy/http-allow/policy.yaml",
					"../../../../../test/cli/test-validating-policy/envoy-deny/policy.yaml",
				},
				HTTPPayloadPaths:  []string{"../../../../../test/cli/test-validating-policy/http-allow/request.json"},
				EnvoyPayloadPaths: []string{"../../../../../test/cli/test-validating-policy/envoy-deny/request.json"},
				PolicyReport:      true,
			},
			expectedReports: []openreportsv1alpha1.Report{{
				Summary: openreportsv1alpha1.ReportSummary{
					Pass: 1,
					Fail: 1,
				},
			}},
		},
	}
	for i, tc := range testcases {
		t.Run(fmt.Sprintf("mixed-authz-%d", i), func(t *testing.T) {
			verifyTestcase(t, tc, compareSummary)
		})
	}
}

func TestCommandWithAuthzPayloadNoResource(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			if strings.Contains(fmt.Sprint(r), "kyverno.http: library version must not be nil") {
				t.Skip("blocked by kyverno-authz: kyverno.http library version panic")
			}
			panic(r)
		}
	}()

	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetArgs([]string{
		"../../../../../test/cli/test-validating-policy/http-allow/policy.yaml",
		"--http-payload",
		"../../../../../test/cli/test-validating-policy/http-allow/request.json",
	})
	err := cmd.Execute()
	assert.NoError(t, err)
}

func TestCommandWithEnvoyPayloadNoResource(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			if strings.Contains(fmt.Sprint(r), "kyverno.http: library version must not be nil") {
				t.Skip("blocked by kyverno-authz: kyverno.http library version panic")
			}
			panic(r)
		}
	}()

	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetArgs([]string{
		"../../../../../test/cli/test-validating-policy/envoy-allow/policy.yaml",
		"--envoy-payload",
		"../../../../../test/cli/test-validating-policy/envoy-allow/request.json",
	})
	err := cmd.Execute()
	assert.NoError(t, err)
}

func TestCommandWithInvalidHTTPPayloadPath(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	cmd.SetArgs([]string{
		"../../../../../test/cli/test-validating-policy/http-allow/policy.yaml",
		"--http-payload",
		"./does-not-exist-http.json",
	})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to parse HTTP payload from")
}

func TestCommandWithInvalidEnvoyPayloadPath(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	cmd.SetArgs([]string{
		"../../../../../test/cli/test-validating-policy/envoy-allow/policy.yaml",
		"--envoy-payload",
		"./does-not-exist-envoy.json",
	})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "failed to parse envoy payload from")
}

func TestCommandWithStdinForPolicyAndResource(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "stdin later in policy and resource paths",
			args: []string{
				"policy.yaml",
				"-",
				"--resource",
				"resource.yaml,-",
			},
		},
		{
			name: "stdin first in policy paths and later in resource paths",
			args: []string{
				"-",
				"policy.yaml",
				"--resource",
				"resource.yaml,-",
			},
		},
		{
			name: "stdin later in policy paths and first in resource paths",
			args: []string{
				"policy.yaml",
				"-",
				"--resource",
				"-,resource.yaml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := Command()
			assert.NotNil(t, cmd)

			b := bytes.NewBufferString("")
			cmd.SetErr(b)

			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			assert.Error(t, err)
			assert.ErrorContains(t, err, "stdin pipe can be used for either policies or resources")
		})
	}
}
