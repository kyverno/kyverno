package apply

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/processor"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/report"
	pkgdeprecations "github.com/kyverno/kyverno/pkg/deprecations"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

		_, _, _, responses, err := tc.config.applyCommandHelper(context.TODO(), os.Stdout)
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
	_, _, _, responses, err := config.applyCommandHelper(context.TODO(), io.Discard)
	assert.NoError(t, err, "should not crash with segfault")
	// K8s-mode policy should be skipped for JSON payloads, so no responses expected
	assert.Equal(t, 0, len(responses), "K8s-mode policies should be skipped for JSON payloads")
}

func Test_Apply_JsonPayload_UnusedK8sPolicy_SkippedCompilation(t *testing.T) {
	// A K8s-mode ValidatingPolicy with an invalid CEL expression should NOT be compiled
	// when evaluating JSON-only payloads, allowing valid JSON policies to succeed.
	tmpDir := t.TempDir()
	invalidK8sPolicy := filepath.Join(tmpDir, "invalid-k8s-policy.yaml")
	err := os.WriteFile(invalidK8sPolicy, []byte(`
apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: invalid-k8s-policy
spec:
  matchConstraints:
    resourceRules:
    - apiGroups:   [""]
      apiVersions: ["v1"]
      operations:  ["CREATE"]
      resources:   ["pods"]
  validations:
  - expression: "this_is_an_invalid_cel_syntax((("
    message: "syntax error"
`), 0o600)
	require.NoError(t, err)

	config := ApplyCommandConfig{
		PolicyPaths: []string{
			invalidK8sPolicy,
			"../../../../../test/cli/test-validating-policy/json-check-dockerfile/policy.yaml",
		},
		JSONPaths:    []string{"../../../../../test/cli/test-validating-policy/json-check-dockerfile/payload.json"},
		PolicyReport: true,
	}
	_, _, _, responses, err := config.applyCommandHelper(context.TODO(), io.Discard)
	assert.NoError(t, err, "unused invalid k8s policy must not fail JSON-only evaluation")
	assert.NotEmpty(t, responses, "JSON policy should have evaluated successfully")
}

func Test_Apply_ContinueOnFail_DefersFailures(t *testing.T) {
	tmpDir := t.TempDir()
	invalidK8sPolicy := filepath.Join(tmpDir, "invalid-k8s-policy.yaml")
	err := os.WriteFile(invalidK8sPolicy, []byte(`
apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: invalid-k8s-policy
spec:
  matchConstraints:
    resourceRules:
    - apiGroups:   [""]
      apiVersions: ["v1"]
      operations:  ["CREATE"]
      resources:   ["pods"]
  validations:
  - expression: "this_is_an_invalid_cel_syntax((("
    message: "syntax error"
`), 0o600)
	require.NoError(t, err)

	invalidContextFile := filepath.Join(tmpDir, "invalid-context.yaml")
	err = os.WriteFile(invalidContextFile, []byte(`: invalid: yaml: [`), 0o600)
	require.NoError(t, err)

	validDeletingPolicy := "../../../../../test/cli/test-deleting-policy/deleting-pod-by-name/policy.yaml"
	validDeletingResource := "../../../../../test/cli/test-deleting-policy/deleting-pod-by-name/resource.yaml"

	validK8sPolicy := "../../../../../test/cli/test-validating-policy/disallow-host-path/policy.yaml"
	validPod := "../../../../../test/cli/test-validating-admission-policy/disallow-host-path/pod1.yaml"

	missingCRDPath := filepath.Join(tmpDir, "nonexistent-crd.yaml")

	testCases := []struct {
		name              string
		config            ApplyCommandConfig
		expectErr         bool
		expectResponses   bool
		expectNoResponses bool
		expectRcError     bool
	}{
		{
			name: "policy compilation failure without continue-on-fail fails fast",
			config: ApplyCommandConfig{
				PolicyPaths:    []string{invalidK8sPolicy, validDeletingPolicy},
				ResourcePaths:  []string{validDeletingResource},
				ContinueOnFail: false,
			},
			expectErr:       true,
			expectResponses: false,
		},
		{
			name: "policy compilation failure with continue-on-fail evaluates subsequent resources",
			config: ApplyCommandConfig{
				PolicyPaths:    []string{invalidK8sPolicy, validDeletingPolicy},
				ResourcePaths:  []string{validDeletingResource},
				ContinueOnFail: true,
			},
			expectErr:       false,
			expectResponses: true,
		},
		{
			name: "context file failure without continue-on-fail fails fast",
			config: ApplyCommandConfig{
				PolicyPaths:    []string{validK8sPolicy},
				ResourcePaths:  []string{validPod},
				ContextPath:    invalidContextFile,
				ContinueOnFail: false,
			},
			expectErr:       true,
			expectResponses: false,
		},
		{
			name: "context file failure with continue-on-fail defers and continues",
			config: ApplyCommandConfig{
				PolicyPaths:    []string{validK8sPolicy},
				ResourcePaths:  []string{validPod},
				ContextPath:    invalidContextFile,
				ContinueOnFail: true,
			},
			expectErr:         false,
			expectNoResponses: true,
		},
		{
			name: "deleting policy context failure without continue-on-fail fails fast",
			config: ApplyCommandConfig{
				PolicyPaths:    []string{validDeletingPolicy},
				ResourcePaths:  []string{validDeletingResource},
				ContextPath:    invalidContextFile,
				ContinueOnFail: false,
			},
			expectErr:       true,
			expectResponses: false,
		},
		{
			name: "deleting policy context failure with continue-on-fail skips deletion and continues",
			config: ApplyCommandConfig{
				PolicyPaths:    []string{validDeletingPolicy},
				ResourcePaths:  []string{validDeletingResource},
				ContextPath:    invalidContextFile,
				ContinueOnFail: true,
			},
			expectErr:         false,
			expectNoResponses: true,
		},
		{
			name: "missing CRD path without continue-on-fail fails fast",
			config: ApplyCommandConfig{
				PolicyPaths:    []string{validK8sPolicy},
				ResourcePaths:  []string{validPod},
				CrdPaths:       []string{missingCRDPath},
				ContinueOnFail: false,
			},
			expectErr:       true,
			expectResponses: false,
		},
		{
			name: "missing CRD path with continue-on-fail defers and continues",
			config: ApplyCommandConfig{
				PolicyPaths:    []string{validK8sPolicy},
				ResourcePaths:  []string{validPod},
				CrdPaths:       []string{missingCRDPath},
				ContinueOnFail: true,
			},
			expectErr:         false,
			expectNoResponses: true,
			expectRcError:     true,
		},
		{
			name: "unused invalid CEL policy does not block HTTP-only evaluation",
			config: ApplyCommandConfig{
				PolicyPaths: []string{
					invalidK8sPolicy,
					"../../../../../test/cli/test-validating-policy/http-allow/policy.yaml",
				},
				HTTPPayloadPaths: []string{"../../../../../test/cli/test-validating-policy/http-allow/request.json"},
				ContinueOnFail:   false,
			},
			expectErr:       false,
			expectResponses: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rc, _, _, responses, err := tc.config.applyCommandHelper(context.TODO(), io.Discard)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tc.expectResponses {
				assert.NotEmpty(t, responses, "expected responses from later valid evaluation")
			}
			if tc.expectNoResponses {
				assert.Empty(t, responses, "missing CRD should prevent resource evaluation")
			}
			if tc.expectRcError {
				assert.True(t, rc != nil && rc.Error > 0, "expected rc.Error > 0")
			}
		})
	}
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

func Test_Apply_ImageVerification_InvalidContext_ErrorCountAndExit(t *testing.T) {
	tmpDir := t.TempDir()
	invalidContextFile := filepath.Join(tmpDir, "invalid-context.yaml")
	err := os.WriteFile(invalidContextFile, []byte(`: invalid: yaml: [`), 0o600)
	require.NoError(t, err)

	cfg := ApplyCommandConfig{
		PolicyPaths:    []string{"../../../../../test/conformance/chainsaw/image-validating-policies/match-conditions/policy.yaml"},
		ResourcePaths:  []string{"../../../../../test/conformance/chainsaw/image-validating-policies/match-conditions/good-pod.yaml"},
		ContextPath:    invalidContextFile,
		ContinueOnFail: true,
	}
	rc, _, _, _, err := cfg.applyCommandHelper(context.TODO(), io.Discard)
	assert.NoError(t, err, "helper should not return error under continue-on-fail")
	assert.Greater(t, rc.Error, 0, "rc.Error should be incremented when image policy context initialization fails")

	exitErr := exit(io.Discard, rc, 0, false)
	assert.Error(t, exitErr, "exit should return error when rc.Error > 0")
}

func Test_Apply_Deleting_InvalidContext_ErrorCountAndExit(t *testing.T) {
	tmpDir := t.TempDir()
	invalidContextFile := filepath.Join(tmpDir, "invalid-context.yaml")
	err := os.WriteFile(invalidContextFile, []byte(`: invalid: yaml: [`), 0o600)
	require.NoError(t, err)

	cfg := ApplyCommandConfig{
		PolicyPaths:    []string{"../../../../../test/cli/test-deleting-policy/deleting-pod-by-name/policy.yaml"},
		ResourcePaths:  []string{"../../../../../test/cli/test-deleting-policy/deleting-pod-by-name/resource.yaml"},
		ContextPath:    invalidContextFile,
		ContinueOnFail: true,
	}
	rc, _, _, _, err := cfg.applyCommandHelper(context.TODO(), io.Discard)
	assert.NoError(t, err, "helper should not return error under continue-on-fail")
	assert.Greater(t, rc.Error, 0, "rc.Error should be incremented when deleting policy context initialization fails")

	exitErr := exit(io.Discard, rc, 0, false)
	assert.Error(t, exitErr, "exit should return error when rc.Error > 0")
}

func Test_Apply_ValidatingPolicy_InvalidContext_ErrorCountAndExit(t *testing.T) {
	tmpDir := t.TempDir()
	invalidContextFile := filepath.Join(tmpDir, "invalid-context.yaml")
	err := os.WriteFile(invalidContextFile, []byte(`: invalid: yaml: [`), 0o600)
	require.NoError(t, err)

	cfg := ApplyCommandConfig{
		PolicyPaths:    []string{"../../../../../test/conformance/chainsaw/validating-policies/validation-rules/accept/policy.yaml"},
		ResourcePaths:  []string{"../../../../../test/conformance/chainsaw/validating-policies/validation-rules/accept/deployment.yaml"},
		ContextPath:    invalidContextFile,
		ContinueOnFail: true,
	}
	rc, _, _, _, err := cfg.applyCommandHelper(context.TODO(), io.Discard)
	assert.NoError(t, err, "helper should not return error under continue-on-fail")
	assert.Greater(t, rc.Error, 0, "rc.Error should be incremented when validating policy context initialization fails")

	exitErr := exit(io.Discard, rc, 0, false)
	assert.Error(t, exitErr, "exit should return error when rc.Error > 0")
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

	_, _, _, responses, err := tc.config.applyCommandHelper(context.TODO(), os.Stdout)
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
	_, _, _, _, err := blocked.applyCommandHelper(context.TODO(), io.Discard)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v1 ClusterPolicy: Kyverno v1.20 removed execution")
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

	_, _, _, responses, err := config.applyCommandHelper(context.TODO(), os.Stdout)
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

	_, _, _, responses, err := config.applyCommandHelper(context.TODO(), os.Stdout)
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

// Test_Apply_ExplainFlagCombinations checks that --explain is rejected next to every flag whose
// stdout is meant for another program, before anything is written, while the combinations whose
// output is for humans keep working.
func Test_Apply_ExplainFlagCombinations(t *testing.T) {
	const (
		policy   = "../../../../../test/cli/test-validating-policy/check-deployment-labels/policy.yaml"
		resource = "../../../../../test/cli/test-validating-policy/check-deployment-labels/deployment1.yaml"
	)
	tests := []struct {
		name    string
		args    []string
		wantErr string // empty means the command must succeed
	}{
		{name: "explain with policy report", args: []string{"--explain", "--policy-report"}, wantErr: "--explain cannot be used with --policy-report"},
		{name: "explain with policy report short flag", args: []string{"--explain", "-p"}, wantErr: "--explain cannot be used with --policy-report"},
		{name: "explain with json policy report", args: []string{"--explain", "--policy-report", "--output-format", "json"}, wantErr: "--explain cannot be used with --policy-report"},
		{name: "explain with generate exceptions", args: []string{"--explain", "--generate-exceptions"}, wantErr: "--explain cannot be used with --generate-exceptions"},
		{name: "explain with stdin", args: []string{"--explain", "--stdin"}, wantErr: "--explain cannot be used with --stdin"},
		{name: "explain with stdin short flag", args: []string{"--explain", "-i"}, wantErr: "--explain cannot be used with --stdin"},
		{name: "explain alone", args: []string{"--explain"}},
		{name: "explain with table", args: []string{"--explain", "--table"}},
		{name: "policy report without explain", args: []string{"--policy-report"}},
		{name: "generate exceptions without explain", args: []string{"--generate-exceptions"}},
		{name: "stdin without explain", args: []string{"--stdin"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := Command()
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs(append([]string{policy, "--resource", resource}, tt.args...))

			err := cmd.Execute()

			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
			assert.Empty(t, stdout.String(), "a rejected combination must fail before writing any output")
		})
	}
}

// Test_Apply_ExplainKeepsTheCostLimit runs the review repro through the apply command: a policy
// that exceeds the CEL cost limit is an error without --explain and must stay an error with it,
// rather than being decided by the trace-only program, which does not enforce the limit.
func Test_Apply_ExplainKeepsTheCostLimit(t *testing.T) {
	const dir = "../../../../../test/cli/test-validating-policy/explain-cost-limit/"
	run := func(explain bool) (*processor.ResultCounts, string) {
		config := ApplyCommandConfig{
			PolicyPaths:   []string{dir + "policy.yaml"},
			ResourcePaths: []string{dir + "resource.yaml"},
			Explain:       explain,
		}
		var out bytes.Buffer
		rc, _, _, _, err := config.applyCommandHelper(context.TODO(), &out)
		require.NoError(t, err)
		return rc, out.String()
	}
	plain, _ := run(false)
	explained, output := run(true)

	assert.Equal(t, 1, plain.Error, "the cost limit stops the expression without --explain")
	assert.Equal(t, 0, plain.Pass)
	assert.Equal(t, plain, explained, "--explain must not change the result")
	assert.Contains(t, output, "VERDICT    ERROR")
	assert.Contains(t, output, "cost limit exceeded")
}

// Test_Apply_Explain runs --explain through the apply command for both input paths: resource
// files (a ValidatingPolicy and a MutatingPolicy) and a JSON payload. With the flag the trace is
// printed; without it nothing trace-shaped appears and the results themselves do not change.
func Test_Apply_Explain(t *testing.T) {
	const base = "../../../../../test/cli/"
	tests := []struct {
		name   string
		config ApplyCommandConfig
		want   []string
		// wantCounts, when set, is what both runs must count
		wantCounts *processor.ResultCounts
	}{{
		name: "validating policy on a resource file",
		config: ApplyCommandConfig{
			PolicyPaths:   []string{base + "test-validating-policy/check-deployment-labels/policy.yaml"},
			ResourcePaths: []string{base + "test-validating-policy/check-deployment-labels/deployment2.yaml"},
		},
		want: []string{"(ValidatingPolicy)", "SCOPE      applied", "VERDICT    FAIL"},
	}, {
		name: "mutating policy on a resource file",
		config: ApplyCommandConfig{
			PolicyPaths:   []string{base + "test-mutating-policy/mutating-label/policy.yaml"},
			ResourcePaths: []string{base + "test-mutating-policy/mutating-label/resource.yaml"},
		},
		want: []string{"(MutatingPolicy)", "SCOPE      applied", "MUTATIONS", "VERDICT    PASS"},
	}, {
		name: "validating policy on a JSON payload",
		config: ApplyCommandConfig{
			PolicyPaths: []string{base + "test-validating-policy/json-check-dockerfile/policy.yaml"},
			JSONPaths:   []string{base + "test-validating-policy/json-check-dockerfile/payload.json"},
		},
		want: []string{"(ValidatingPolicy)", "evaluated against a JSON payload", "VERDICT"},
	}, {
		// the mutated resource's output ends without a newline, so a trace printed after it, of
		// either policy type, must still start on its own line
		name: "mutating and validating policies on the same resources",
		config: ApplyCommandConfig{
			PolicyPaths: []string{
				base + "test-mutating-policy/mutating-label/policy.yaml",
				base + "test-validating-policy/check-deployment-labels/policy.yaml",
			},
			ResourcePaths: []string{base + "test-mutating-policy/mutating-label/resource.yaml"},
		},
		want: []string{"(MutatingPolicy)", "(ValidatingPolicy)", "Mutation has been applied successfully."},
	}, {
		// two pods: test-nginx-1 meets both conditions, test-nginx-2 fails the second
		name: "deleting policy on a resource file",
		config: ApplyCommandConfig{
			PolicyPaths:   []string{base + "test-deleting-policy/deleting-pod-by-name/policy.yaml"},
			ResourcePaths: []string{base + "test-deleting-policy/deleting-pod-by-name/resource.yaml"},
		},
		want: []string{
			"(DeletingPolicy)",
			"(deletion scan, so operations are not checked)",
			"VERDICT    PASS     conditions held: the resource would be deleted",
			`VERDICT    FAIL     condition "pod-name" is false: the resource is kept`,
		},
	}, {
		name: "deleting policy on a JSON payload",
		config: ApplyCommandConfig{
			PolicyPaths: []string{base + "test-deleting-policy/deleting-json/policy.yaml"},
			JSONPaths:   []string{base + "test-deleting-policy/deleting-json/payload.json"},
		},
		want: []string{"(DeletingPolicy)", "evaluated against a JSON payload", "VERDICT"},
	}, {
		name: "generating policy with a generate expression",
		config: ApplyCommandConfig{
			PolicyPaths:   []string{base + "test-generating-policy/data/generate-cm/policy.yaml"},
			ResourcePaths: []string{base + "test-generating-policy/data/generate-cm/resource.yaml"},
		},
		want: []string{
			"(GeneratingPolicy)",
			"GENERATE            generate[0] (expression): generator.apply(variables.nsName, variables.configmap)  ->  true",
			"generated ConfigMap test-ns/zk-kafka-address",
			"VERDICT    PASS     generated 1 resource",
		},
	}, {
		name: "generating policy with a template",
		config: ApplyCommandConfig{
			PolicyPaths:   []string{base + "test-generating-policy/template/generate-cm-interpolated/policy.yaml"},
			ResourcePaths: []string{base + "test-generating-policy/template/generate-cm-interpolated/resource.yaml"},
		},
		want: []string{
			"(GeneratingPolicy)",
			"GENERATE            generate[0] (template)",
			"generated ConfigMap test-ns/test-ns-config",
			"VERDICT    PASS     generated 1 resource",
		},
	}, {
		// the generate expression reads a label the namespace does not have
		name: "generating policy whose generate expression fails",
		config: ApplyCommandConfig{
			PolicyPaths:   []string{base + "test-generating-policy/explain-generate-error/policy.yaml"},
			ResourcePaths: []string{base + "test-generating-policy/explain-generate-error/resource.yaml"},
		},
		want: []string{
			"(GeneratingPolicy)",
			"GENERATE   ERROR    generate[0] (expression): no such key: labels",
			"object.metadata.labels  ->  ERROR: no such key: labels",
			"VERDICT    ERROR    no such key: labels",
		},
		wantCounts: &processor.ResultCounts{Error: 1},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := func(explain bool) (string, *processor.ResultCounts) {
				config := tt.config
				config.Explain = explain
				var out bytes.Buffer
				rc, _, _, _, err := config.applyCommandHelper(context.TODO(), &out)
				require.NoError(t, err)
				return out.String(), rc
			}

			explained, explainedCounts := run(true)
			for _, want := range tt.want {
				assert.Contains(t, explained, want)
			}
			for _, line := range strings.Split(explained, "\n") {
				if i := strings.Index(line, "Policy:   "); i > 0 {
					t.Errorf("a trace header must start its own line, got %q", line)
				}
			}

			plain, plainCounts := run(false)
			assert.NotContains(t, plain, "VERDICT ", "no trace without --explain")
			assert.NotContains(t, plain, "SCOPE ", "no trace without --explain")
			assert.Equal(t, plainCounts, explainedCounts, "--explain must not change the results")
			if tt.wantCounts != nil {
				assert.Equal(t, tt.wantCounts, plainCounts)
			}
		})
	}
}

// Test_Apply_ExplainThroughTheCommand runs `apply --explain` the way a user does, through the
// Cobra command and its flag parsing rather than by setting ApplyCommandConfig.Explain, and
// checks the trace reaches the command's output. Without the flag no trace is printed, and the
// flag never changes whether the command succeeds.
func Test_Apply_ExplainThroughTheCommand(t *testing.T) {
	const base = "../../../../../test/cli/"
	tests := []struct {
		name     string
		policy   string
		resource string
		want     []string
		// wantErr is a command that fails outright: --explain must fail it the same way, and
		// no trace is expected
		wantErr bool
	}{{
		name:     "validating policy",
		policy:   base + "test-validating-policy/check-deployment-labels/policy.yaml",
		resource: base + "test-validating-policy/check-deployment-labels/deployment2.yaml",
		want:     []string{"(ValidatingPolicy)", "SCOPE      applied", "VERDICT    FAIL"},
	}, {
		name:     "mutating policy",
		policy:   base + "test-mutating-policy/mutating-label/policy.yaml",
		resource: base + "test-mutating-policy/mutating-label/resource.yaml",
		want:     []string{"(MutatingPolicy)", "SCOPE      applied", "MUTATIONS", "VERDICT    PASS"},
	}, {
		name:     "deleting policy",
		policy:   base + "test-deleting-policy/deleting-pod-by-name/policy.yaml",
		resource: base + "test-deleting-policy/deleting-pod-by-name/resource.yaml",
		want:     []string{"(DeletingPolicy)", "VERDICT    PASS     conditions held: the resource would be deleted"},
	}, {
		name:     "missing resource file",
		policy:   base + "test-validating-policy/check-deployment-labels/policy.yaml",
		resource: base + "test-validating-policy/check-deployment-labels/does-not-exist.yaml",
		wantErr:  true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := func(extra ...string) (string, error) {
				cmd := Command()
				var out bytes.Buffer
				cmd.SetOut(&out)
				cmd.SetErr(&out)
				cmd.SetArgs(append([]string{tt.policy, "--resource", tt.resource}, extra...))
				err := cmd.Execute()
				return out.String(), err
			}

			explained, explainedErr := run("--explain")
			if tt.wantErr {
				_, plainErr := run()
				require.Error(t, plainErr)
				require.Error(t, explainedErr, "--explain must not hide a command error")
				assert.Equal(t, plainErr.Error(), explainedErr.Error())
				return
			}
			for _, want := range tt.want {
				assert.Contains(t, explained, want)
			}

			plain, plainErr := run()
			assert.NotContains(t, plain, "VERDICT ", "no trace without --explain")
			assert.NotContains(t, plain, "SCOPE ", "no trace without --explain")
			assert.Equal(t, plainErr == nil, explainedErr == nil, "--explain must not change whether the command succeeds")
		})
	}
}

// Test_Apply_MutatingPolicyOutputFile checks what --output writes: only the patched target for a policy
// with targetMatchConstraints, never its trigger, while an ordinary MutatingPolicy still writes the
// resources it mutated inline.
func Test_Apply_MutatingPolicyOutputFile(t *testing.T) {
	const base = "../../../../../test/cli/test-mutating-policy/"
	tests := []struct {
		name            string
		args            []string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "target only policy writes the patched target",
			args: []string{
				base + "mutate-existing-secret-trigger-deployment-target/policy.yaml",
				"--resource", base + "mutate-existing-secret-trigger-deployment-target/trigger.yaml",
				"--target-resource", base + "mutate-existing-secret-trigger-deployment-target/target.yaml",
			},
			wantContains:    []string{"kind: Deployment", "name: test-deploy", `mutated: "true"`, "replicas: 3"},
			wantNotContains: []string{"kind: Secret"},
		},
		{
			name: "ordinary policy writes the mutated resource",
			args: []string{
				base + "mutating-label/policy.yaml",
				"--resource", base + "mutating-label/resource.yaml",
			},
			wantContains: []string{"kind: Deployment", "name: dev-deploy-2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "mutated.yaml")
			cmd := Command()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(append(tt.args, "--output", output))
			require.NoError(t, cmd.Execute())

			written, err := os.ReadFile(output)
			require.NoError(t, err)
			// no empty document ahead of the first resource, the trigger is not written as a blank entry
			assert.True(t, strings.HasPrefix(string(written), "apiVersion: "), "unexpected start of output: %q", string(written)[:min(len(written), 40)])
			for _, want := range tt.wantContains {
				assert.Contains(t, string(written), want)
			}
			for _, unwanted := range tt.wantNotContains {
				assert.NotContains(t, string(written), unwanted)
			}
		})
	}
}

// Test_Apply_MutatingPolicyTargetResources checks that a MutatingPolicy with targetMatchConstraints mutates
// the resources passed with --target-resource and never the trigger itself, and that a target whose
// mutation fails is reported as an error without patching anything.
func Test_Apply_MutatingPolicyTargetResources(t *testing.T) {
	dir := "../../../../../test/cli/test-mutating-policy/mutate-existing-secret-trigger-deployment-target"
	policy, err := os.ReadFile(dir + "/policy.yaml")
	require.NoError(t, err)
	// the same policy, but its mutation fails at evaluation time on every selected target
	failing := strings.Replace(string(policy), "replicas: 3", "replicas: object.spec.missing.count", 1)
	require.NotEqual(t, string(policy), failing)
	failingPolicy := filepath.Join(t.TempDir(), "failing-policy.yaml")
	require.NoError(t, os.WriteFile(failingPolicy, []byte(failing), 0o600))
	// the same policy, but its matchConditions reject the trigger, so it must not mutate any target
	conditioned := strings.Replace(string(policy), "  targetMatchConstraints:", "  matchConditions:\n  - name: other-secret-only\n    expression: \"object.metadata.name == 'other-secret'\"\n  targetMatchConstraints:", 1)
	require.NotEqual(t, string(policy), conditioned)
	conditionedPolicy := filepath.Join(t.TempDir(), "conditioned-policy.yaml")
	require.NoError(t, os.WriteFile(conditionedPolicy, []byte(conditioned), 0o600))
	// the rejected policy under its own name, to load next to the matching one
	const notTriggeredName = "not-triggered-by-this-secret"
	notTriggeredPolicy := filepath.Join(t.TempDir(), "not-triggered-policy.yaml")
	require.NoError(t, os.WriteFile(notTriggeredPolicy, []byte(strings.Replace(conditioned, "name: mutate-existing-deployment-from-secret", "name: "+notTriggeredName, 1)), 0o600))
	// a trigger the policy's matchConstraints do not select
	configMapTrigger := filepath.Join(t.TempDir(), "configmap.yaml")
	require.NoError(t, os.WriteFile(configMapTrigger, []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test-config\n  namespace: test-ns\ndata:\n  key: value\n"), 0o600))

	tests := []struct {
		name            string
		policy          string
		extraPolicies   []string
		trigger         string
		targetResources []string
		wantPass        int
		wantError       int
	}{
		{
			name:            "with target resource",
			policy:          dir + "/policy.yaml",
			targetResources: []string{dir + "/target.yaml"},
			wantPass:        1,
		},
		{
			name:   "without target resource",
			policy: dir + "/policy.yaml",
		},
		{
			name:            "with target resource whose mutation fails",
			policy:          failingPolicy,
			targetResources: []string{dir + "/target.yaml"},
			wantError:       1,
		},
		{
			name:            "with a trigger the policy does not match",
			policy:          dir + "/policy.yaml",
			trigger:         configMapTrigger,
			targetResources: []string{dir + "/target.yaml"},
		},
		{
			name:            "with a trigger rejected by matchConditions",
			policy:          conditionedPolicy,
			targetResources: []string{dir + "/target.yaml"},
		},
		{
			name:            "with a second policy the trigger does not match",
			policy:          dir + "/policy.yaml",
			extraPolicies:   []string{notTriggeredPolicy},
			targetResources: []string{dir + "/target.yaml"},
			wantPass:        1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trigger := tt.trigger
			if trigger == "" {
				trigger = dir + "/trigger.yaml"
			}
			config := ApplyCommandConfig{
				PolicyPaths:         append([]string{tt.policy}, tt.extraPolicies...),
				ResourcePaths:       []string{trigger},
				TargetResourcePaths: tt.targetResources,
			}
			out := &bytes.Buffer{}
			rc, _, _, responses, err := config.applyCommandHelper(context.TODO(), out)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantPass, rc.Pass)
			assert.Equal(t, tt.wantError, rc.Error)
			assert.Equal(t, 0, rc.Fail)

			var targetMutated, targetErrored bool
			for _, response := range responses {
				assert.NotEqual(t, notTriggeredName, response.Policy().GetName(), "a policy whose trigger rules do not match must not mutate targets")
				if response.Policy().GetName() != "mutate-existing-deployment-from-secret" {
					continue
				}
				if response.PatchedResource.Object != nil {
					assert.Equal(t, response.Resource.Object, response.PatchedResource.Object, "trigger must not be patched")
				}
				for _, rule := range response.PolicyResponse.Rules {
					assert.Equal(t, "Deployment", response.Resource.GetKind(), "rule %q evaluated against the trigger: %s", rule.Name(), rule.Message())
					patched, _, _ := rule.PatchedTarget()
					switch rule.Status() {
					case engineapi.RuleStatusPass:
						if assert.NotNil(t, patched) {
							assert.Equal(t, "Deployment", patched.GetKind())
							assert.Equal(t, "true", patched.GetLabels()["mutated"])
							targetMutated = true
						}
					case engineapi.RuleStatusError:
						assert.Nil(t, patched, "a failed mutation must not produce a patched target")
						assert.Contains(t, rule.Message(), "no such key")
						targetErrored = true
					default:
						t.Errorf("unexpected rule status %s: %s", rule.Status(), rule.Message())
					}
				}
			}
			assert.Equal(t, tt.wantPass > 0, targetMutated)
			assert.Equal(t, tt.wantError > 0, targetErrored)
			assert.Equal(t, tt.wantPass > 0, strings.Contains(out.String(), "patched targets"))
			assert.NotContains(t, out.String(), "kind: Secret", "trigger must not be printed as mutated")
		})
	}
}

// Test_Apply_ExplainImageValidatingPolicy runs an ImageValidatingPolicy with --explain: a pod
// that passes, one that fails, and one a match condition skips. Its images do not match
// matchImageReferences, so nothing is fetched or verified. The trace is printed, and the results
// are exactly the ones without --explain.
func Test_Apply_ExplainImageValidatingPolicy(t *testing.T) {
	const dir = "../../../../../test/cli/test-image-validating-policy/explain/"
	run := func(explain bool) (*processor.ResultCounts, string) {
		config := ApplyCommandConfig{
			PolicyPaths:   []string{dir + "policy.yaml"},
			ResourcePaths: []string{dir + "resources.yaml"},
			Explain:       explain,
		}
		var out bytes.Buffer
		rc, _, _, _, err := config.applyCommandHelper(context.TODO(), &out)
		require.NoError(t, err)
		return rc, out.String()
	}
	plain, plainOut := run(false)
	explained, output := run(true)

	assert.Equal(t, 1, plain.Pass)
	assert.Equal(t, 1, plain.Fail)
	assert.Equal(t, plain, explained, "--explain must not change the results")
	assert.NotContains(t, plainOut, "VERDICT ", "no trace without --explain")

	for _, want := range []string{
		"Policy:   check-images (ImageValidatingPolicy)",
		"Resource: Pod/with-team (namespace: prod)",
		"IMAGES     skipped  containers: docker.io/library/nginx:1.27  (not matched by matchImageReferences)",
		"VARIABLES           team: object.metadata.?labels.?team.orValue('')  ->  payments",
		"VERDICT    PASS     all 2 validations passed",
		"VALIDATION FAIL     [1] variables.team != ''  ->  false",
		`message: "pods need a team label"`,
		`VERDICT    SKIP     match condition "not-kube-system" did not pass, so the policy was skipped`,
	} {
		assert.Contains(t, output, want)
	}
}

// Test_Apply_ImageValidatingPolicyJSONSkippedByMatchCondition is a JSON payload a JSON-mode
// policy's match condition excludes. Evaluating it returns a nil result, which the apply command
// used to read and crash on; it must report nothing, and with --explain print the skip.
func Test_Apply_ImageValidatingPolicyJSONSkippedByMatchCondition(t *testing.T) {
	const dir = "../../../../../test/cli/test-image-validating-policy/explain/"
	for _, explain := range []bool{false, true} {
		config := ApplyCommandConfig{
			PolicyPaths: []string{dir + "json-policy.yaml"},
			JSONPaths:   []string{dir + "payload.json"},
			Explain:     explain,
		}
		var out bytes.Buffer
		var rc *processor.ResultCounts
		require.NotPanics(t, func() {
			var err error
			rc, _, _, _, err = config.applyCommandHelper(context.TODO(), &out)
			require.NoError(t, err)
		})
		assert.Equal(t, processor.ResultCounts{}, *rc, "a skipped payload is not counted (explain=%v)", explain)
		if explain {
			assert.Contains(t, out.String(), `VERDICT    SKIP     match condition "is-production" did not pass`)
			assert.Contains(t, out.String(), "evaluated against a JSON payload")
		}
	}
}
