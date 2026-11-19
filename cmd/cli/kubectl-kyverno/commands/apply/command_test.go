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

	fatihcolor "github.com/fatih/color"
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/processor"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/report"
	pkgdeprecations "github.com/kyverno/kyverno/pkg/deprecations"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

func TestMain(m *testing.M) {
	log.SetLogger(logr.Discard())
	m.Run()
}

func TestPrintSkippedAndInvalidPolicies(t *testing.T) {
	origNoColor := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = origNoColor
		color.Init(origNoColor)
	})

	testData := SkippedInvalidPolicies{
		skipped: []PolicyDiagnostic{
			{
				name:   "skipped-policy",
				reason: "missing variable `request.operation`",
			},
			{
				name:   "skipped-policy-no-reason",
				reason: "",
			},
		},
		invalid: []PolicyDiagnostic{
			{
				name:   "invalid-policy",
				reason: "failed to compile policy invalid-policy (spec.matchConditions[0].expression: Forbidden: variables cannot be referenced)",
			},
			{
				name:   "invalid-policy-no-reason",
				reason: "",
			},
			{
				name:   "policies/team-a/typo.yaml",
				reason: "failed to parse YAML",
			},
		},
	}

	t.Run("color disabled", func(t *testing.T) {
		fatihcolor.NoColor = true
		color.Init(true)
		var out bytes.Buffer
		printSkippedAndInvalidPolicies(&out, testData)

		printed := out.String()
		assert.Contains(t, printed, "Policies Skipped:")
		assert.Contains(t, printed, "1. skipped-policy: missing variable `request.operation`")
		assert.Contains(t, printed, "2. skipped-policy-no-reason")
		assert.Contains(t, printed, "Invalid Policies:")
		assert.Contains(t, printed, "1. invalid-policy: failed to compile policy invalid-policy")
		assert.Contains(t, printed, "2. invalid-policy-no-reason")
		assert.Contains(t, printed, "3. policies/team-a/typo.yaml: failed to parse YAML")
		assert.Contains(t, printed, "variables cannot be referenced")
		assert.NotContains(t, printed, "\x1b[")
	})

	t.Run("color enabled", func(t *testing.T) {
		fatihcolor.NoColor = false
		color.Init(false)
		var out bytes.Buffer
		printSkippedAndInvalidPolicies(&out, testData)

		printed := out.String()
		assert.Contains(t, printed, "\x1b[33;1mPolicies Skipped:\x1b[0;22m")
		assert.Contains(t, printed, "\x1b[36;1mskipped-policy\x1b[0;22m: missing variable `request.operation`")
		assert.Contains(t, printed, "\x1b[36;1mskipped-policy-no-reason\x1b[0;22m")
		assert.Contains(t, printed, "\x1b[31;1mInvalid Policies:\x1b[0;22m")
		assert.Contains(t, printed, "\x1b[36;1minvalid-policy\x1b[0;22m: failed to compile policy invalid-policy")
		assert.Contains(t, printed, "\x1b[36;1minvalid-policy-no-reason\x1b[0;22m")
		assert.Contains(t, printed, "\x1b[36;1mpolicies/team-a/typo.yaml\x1b[0;22m: failed to parse YAML")
	})
}

func TestPrintViolations(t *testing.T) {
	rc := &processor.ResultCounts{
		Pass:  1,
		Fail:  2,
		Warn:  3,
		Error: 4,
		Skip:  5,
	}

	origNoColor := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = origNoColor
		color.Init(origNoColor)
	})

	t.Run("color disabled", func(t *testing.T) {
		fatihcolor.NoColor = true
		color.Init(true)
		var out bytes.Buffer
		printViolations(&out, rc)
		expected := "\npass: 1, fail: 2, warn: 3, error: 4, skip: 5 \n"
		assert.Equal(t, expected, out.String())
		assert.NotContains(t, out.String(), "\x1b[")
	})

	t.Run("color enabled", func(t *testing.T) {
		fatihcolor.NoColor = false
		color.Init(false)
		var out bytes.Buffer
		printViolations(&out, rc)
		expected := "\npass: \x1b[32;1m1\x1b[0;22m, fail: \x1b[31;1m2\x1b[0;22m, warn: \x1b[33;1m3\x1b[0;22m, error: \x1b[31;1m4\x1b[0;22m, skip: \x1b[36;1m5\x1b[0;22m \n"
		assert.Equal(t, expected, out.String())
	})
}

func TestApplyJSONMutationUsesExistingOutput(t *testing.T) {
	base := "../../../../../test/cli/test-mutating-policy/json-document-mutation/"
	output := t.TempDir()
	config := &ApplyCommandConfig{
		PolicyPaths: []string{base + "policy.yaml"},
		JSONPaths:   []string{base + "input.json", base + "nested/input.json"},
		OutputDir:   output,
	}
	rc, _, _, _, err := config.applyCommandHelper(context.Background(), io.Discard)
	require.NoError(t, err)
	require.Equal(t, 4, rc.Pass)
	require.Zero(t, rc.Error)
	content, err := os.ReadFile(filepath.Join(output, "add-region-mutated.yaml"))
	require.NoError(t, err)
	require.Equal(t, 2, strings.Count(string(content), "region: west"))
	require.Contains(t, string(content), "9007199254740993")
}

func TestApplyJSONMutationRejectsNonObject(t *testing.T) {
	for _, root := range []string{"[]", "null", "true"} {
		t.Run(root, func(t *testing.T) {
			input := filepath.Join(t.TempDir(), "input.json")
			require.NoError(t, os.WriteFile(input, []byte(root), 0o600))
			config := &ApplyCommandConfig{
				PolicyPaths: []string{"../../../../../test/cli/test-mutating-policy/json-document-mutation/policy.yaml"},
				JSONPaths:   []string{input},
			}
			_, _, _, _, err := config.applyCommandHelper(context.Background(), io.Discard)
			require.ErrorContains(t, err, "top-level object")
		})
	}
}

func TestApplyJSONMutationSkipsUnusedKubernetesPolicy(t *testing.T) {
	base := "../../../../../test/cli/test-mutating-policy/json-document-mutation/"
	policies, err := os.ReadFile(base + "policy.yaml")
	require.NoError(t, err)
	policies = append(policies, []byte(`
---
apiVersion: policies.kyverno.io/v1
kind: MutatingPolicy
metadata:
  name: unused-kubernetes-policy
spec:
  mutations:
  - patchType: JSONPatch
    jsonPatch:
      expression: 'undefinedVariable'
`)...)
	path := filepath.Join(t.TempDir(), "policies.yaml")
	require.NoError(t, os.WriteFile(path, policies, 0o600))
	config := &ApplyCommandConfig{PolicyPaths: []string{path}, JSONPaths: []string{base + "input.json"}}
	rc, _, _, _, err := config.applyCommandHelper(context.Background(), io.Discard)
	require.NoError(t, err)
	require.Equal(t, 2, rc.Pass)
	require.Zero(t, rc.Error)
}

func TestApplyResourceMutationSkipsUnusedJSONPolicy(t *testing.T) {
	for _, mutation := range []struct {
		name string
		yaml string
	}{
		{"invalid expression", `- patchType: JSONPatch
    jsonPatch:
      expression: 'undefinedVariable'`},
		{"unsupported patch type", `- patchType: ApplyConfiguration
    applyConfiguration:
      expression: 'Object{}'`},
	} {
		for _, inputs := range []struct {
			name               string
			withResourcePolicy bool
			withJSONInput      bool
		}{
			{"resource only", true, false},
			{"no applicable policies", false, false},
			{"applicable invalid JSON policy", false, true},
		} {
			t.Run(mutation.name+"/"+inputs.name, func(t *testing.T) {
				dir := t.TempDir()
				policy := filepath.Join(dir, "policies.yaml")
				policies := fmt.Sprintf(`
apiVersion: policies.kyverno.io/v1
kind: MutatingPolicy
metadata:
  name: invalid-json-policy
spec:
  evaluation:
    mode: JSON
  mutations:
  %s
`, mutation.yaml)
				if inputs.withResourcePolicy {
					policies += `
---
apiVersion: policies.kyverno.io/v1
kind: MutatingPolicy
metadata:
  name: mutate-resource
spec:
  mutations:
  - patchType: JSONPatch
    jsonPatch:
      expression: '[JSONPatch{op: "add", path: "/data/mutated", value: "yes"}]'
`
				}
				require.NoError(t, os.WriteFile(policy, []byte(policies), 0o600))
				resource := filepath.Join(dir, "resource.yaml")
				require.NoError(t, os.WriteFile(resource, []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: input
data: {}
`), 0o600))
				config := &ApplyCommandConfig{PolicyPaths: []string{policy}, ResourcePaths: []string{resource}}
				if inputs.withJSONInput {
					config.ResourcePaths = nil
					config.JSONPaths = []string{"../../../../../test/cli/test-mutating-policy/json-document-mutation/input.json"}
				}
				rc, _, _, responses, err := config.applyCommandHelper(context.Background(), io.Discard)
				if inputs.withJSONInput {
					require.ErrorContains(t, err, "failed to compile policy invalid-json-policy")
					return
				}
				require.NoError(t, err)
				require.Zero(t, rc.Error)
				if inputs.withResourcePolicy {
					require.Equal(t, 1, rc.Pass)
					require.Len(t, responses, 1)
					require.Equal(t, map[string]any{"mutated": "yes"}, responses[0].PatchedResource.Object["data"])
				} else {
					require.Zero(t, rc.Pass)
					require.Empty(t, responses)
				}
			})
		}
	}
}

func TestApplyJSONMutationDoesNotMutateKubernetesObject(t *testing.T) {
	input := filepath.Join(t.TempDir(), "resource.json")
	require.NoError(t, os.WriteFile(input, []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"input"},"id":1}`), 0o600))
	policy := filepath.Join(t.TempDir(), "json-policy.yaml")
	require.NoError(t, os.WriteFile(policy, []byte(`
apiVersion: policies.kyverno.io/v1
kind: MutatingPolicy
metadata:
  name: json-only
spec:
  evaluation:
    mode: JSON
  mutations:
  - patchType: JSONPatch
    jsonPatch:
      expression: '[JSONPatch{op: "add", path: "/region", value: "west"}]'
`), 0o600))
	config := &ApplyCommandConfig{
		PolicyPaths:   []string{policy},
		ResourcePaths: []string{input},
	}
	rc, resources, _, responses, err := config.applyCommandHelper(context.Background(), io.Discard)
	require.NoError(t, err)
	require.Zero(t, rc.Pass)
	require.Zero(t, rc.Fail)
	require.Zero(t, rc.Error)
	require.Len(t, resources, 1)
	require.Equal(t, "ConfigMap", resources[0].GetKind())
	require.Equal(t, "input", resources[0].GetName())
	require.Empty(t, responses)
	require.NotContains(t, resources[0].Object, "region")
}

func TestApplyJSONMutationSkipDoesNotPrintSuccess(t *testing.T) {
	base := "../../../../../test/cli/test-mutating-policy/json-document-mutation/"
	for _, tc := range []struct {
		name       string
		match      string
		expression string
	}{
		{
			name:       "match condition",
			match:      "false",
			expression: `[JSONPatch{op: "add", path: "/region", value: "west"}]`,
		},
		{
			name:       "failed patch test",
			expression: `[JSONPatch{op: "test", path: "/id", value: 1}, JSONPatch{op: "add", path: "/region", value: "west"}]`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := filepath.Join(t.TempDir(), "skip-policy.yaml")
			matchCondition := ""
			if tc.match != "" {
				matchCondition = fmt.Sprintf(`
  matchConditions:
  - name: excluded
    expression: %q`, tc.match)
			}
			require.NoError(t, os.WriteFile(policy, []byte(fmt.Sprintf(`
apiVersion: policies.kyverno.io/v1
kind: MutatingPolicy
metadata:
  name: skip-json-mutation
spec:
  evaluation:
    mode: JSON
%s
  mutations:
  - patchType: JSONPatch
    jsonPatch:
      expression: %q
`, matchCondition, tc.expression)), 0o600))
			var out bytes.Buffer
			config := &ApplyCommandConfig{PolicyPaths: []string{policy}, JSONPaths: []string{base + "input.json"}}
			rc, _, _, responses, err := config.applyCommandHelper(context.Background(), &out)
			require.NoError(t, err)
			require.Equal(t, 1, rc.Skip)
			require.Zero(t, rc.Pass)
			require.Len(t, responses, 1)
			require.NotContains(t, out.String(), "Mutation has been applied successfully")
			require.NotContains(t, out.String(), "region: west")
		})
	}
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
		name: "mutating policy on a JSON payload",
		config: ApplyCommandConfig{
			PolicyPaths: []string{base + "test-mutating-policy/json-document-mutation/policy.yaml"},
			JSONPaths:   []string{base + "test-mutating-policy/json-document-mutation/input.json"},
		},
		want:       []string{"add-region (MutatingPolicy)", "evaluated against a JSON payload", "MUTATIONS", "VERDICT    PASS"},
		wantCounts: &processor.ResultCounts{Pass: 2},
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

// Test_Apply_ExplainHidesSecretData runs policies that read a Secret, given by a context file,
// with --explain: the trace prints neither the Secret's data, nor a value worked out from it, nor
// an error that quotes it, while values that do not come from a Secret are still printed. The
// results are the ones without --explain.
func Test_Apply_ExplainHidesSecretData(t *testing.T) {
	const base = "../../../../../test/cli/"
	tests := []struct {
		name string
		dir  string
		want []string
	}{{
		name: "a validating policy reads a Secret into variables",
		dir:  base + "test-validating-policy/explain-secret/",
		want: []string{
			"VARIABLES           password: resource.Get('v1', 'secrets', 'kyverno', 'db-credentials').data.password  ->  (redacted)",
			"VARIABLES           decoded: string(base64.decode(variables.password))  ->  (redacted)",
			"VARIABLES           team: object.metadata.?labels.?team.orValue('')  ->  payments",
			`message: "the password is too short"`,
		},
	}, {
		name: "an image validating policy fails on a Secret's data",
		dir:  base + "test-image-validating-policy/explain-secret/",
		want: []string{
			"VERDICT    ERROR    timestamp(resource.Get('v1', 'secrets', 'kyverno', 'db-credentials').data.password) < timestamp('2026-01-01T00:00:00Z')  ->  (redacted)",
			`message: "(redacted)"`,
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := func(explain bool) (*processor.ResultCounts, string) {
				config := ApplyCommandConfig{
					PolicyPaths:   []string{tt.dir + "policy.yaml"},
					ResourcePaths: []string{tt.dir + "resources.yaml"},
					ContextPath:   tt.dir + "context.yaml",
					Explain:       explain,
				}
				var out bytes.Buffer
				rc, _, _, _, err := config.applyCommandHelper(context.TODO(), &out)
				require.NoError(t, err)
				return rc, out.String()
			}
			plain, _ := run(false)
			explained, output := run(true)
			assert.Equal(t, plain, explained, "--explain must not change the results")
			assert.NotContains(t, output, "czNjcjN0LXA0c3M=", "the Secret's data")
			assert.NotContains(t, output, "s3cr3t-p4ss", "the Secret's data, decoded")
			for _, want := range tt.want {
				assert.Contains(t, output, want)
			}
		})
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

func TestCommandWithColorAndRemoveColor(t *testing.T) {
	origNoColor := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = origNoColor
		color.Init(origNoColor)
	})

	policyPath := "../../../../../test/cli/test-validating-policy/disallow-host-path/policy.yaml"
	resourcePath := "../../../../../test/cli/test-validating-policy/disallow-host-path/pod2.yaml"

	t.Run("color enabled by default", func(t *testing.T) {
		fatihcolor.NoColor = false
		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath})
		err := cmd.Execute()
		assert.Error(t, err)

		output := out.String()
		assert.Contains(t, output, "\x1b[31;1mfailed:\x1b[0;22m")
		assert.Contains(t, output, "\x1b[36;1mdisallow-host-path\x1b[0;22m")
		assert.Contains(t, output, "\x1b[31;1m1\x1b[0;22m")
	})

	t.Run("remove color with flag", func(t *testing.T) {
		fatihcolor.NoColor = false
		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--remove-color"})
		err := cmd.Execute()
		assert.Error(t, err)

		output := out.String()
		assert.Contains(t, output, "failed:")
		assert.Contains(t, output, "policy disallow-host-path -> resource default/Pod/bad-pod failed:")
		assert.NotContains(t, output, "\x1b[")
	})

	t.Run("suppress color in policy-report structured output", func(t *testing.T) {
		fatihcolor.NoColor = false
		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--policy-report"})
		err := cmd.Execute()
		assert.Error(t, err)

		output := out.String()
		assert.Contains(t, output, "kind: ClusterReport")
		assert.NotContains(t, output, "\x1b[")
	})

	t.Run("suppress color in generate-exceptions structured output", func(t *testing.T) {
		fatihcolor.NoColor = false
		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--generate-exceptions"})
		err := cmd.Execute()
		assert.Error(t, err)

		output := out.String()
		assert.Contains(t, output, "kind: PolicyException")
		assert.NotContains(t, output, "\x1b[")

		var parsed map[string]any
		trimmed := bytes.TrimPrefix(out.Bytes(), []byte("---\n"))
		require.NoError(t, yaml.Unmarshal(trimmed, &parsed))
	})

	t.Run("suppress color in stdin mode", func(t *testing.T) {
		fatihcolor.NoColor = false
		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--stdin"})
		err := cmd.Execute()
		assert.Error(t, err)

		output := out.String()
		assert.Contains(t, output, "failed:")
		assert.NotContains(t, output, "\x1b[")
	})
}

func Test_Apply_PolicyReport_DeprecatedUserInfoWarningNoColor(t *testing.T) {
	origNoColor := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = origNoColor
		color.Init(origNoColor)
	})
	fatihcolor.NoColor = false
	color.Init(false)

	policyPath := "../../../../../test/cli/test-validating-policy/disallow-host-path/policy.yaml"
	resourcePath := "../../../../../test/cli/test-validating-policy/disallow-host-path/pod2.yaml"
	userInfoPath := "../../_testdata/user-infos/valid.yaml"

	cmd := Command()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "-u", userInfoPath, "-p"})
	err := cmd.Execute()
	assert.Error(t, err)

	output := out.String()
	assert.Contains(t, output, "WARNING: user infos file")
	assert.Contains(t, output, "uses a deprecated schema")
	assert.Contains(t, output, "kind: ClusterReport")
	assert.NotContains(t, output, "\x1b[")
}

// TestAuditWarnWithPolicyReport verifies that applying an audit-only policy with --audit-warn
// and --policy-report reports status "warn" and respects the --warn-exit-code flag.
func TestAuditWarnWithPolicyReport(t *testing.T) {
	policyPath := "../../../../../test/conformance/chainsaw/cli/apply/warn-exit-code/policy.yaml"
	resourcePath := "../../../../../test/conformance/chainsaw/cli/apply/warn-exit-code/resource.yaml"

	t.Run("audit-warn with policy-report produces warn result and exit 0", func(t *testing.T) {
		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--audit-warn", "--policy-report"})
		err := cmd.Execute()
		assert.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "kind: ClusterReport")
		assert.Contains(t, output, "result: warn")
		assert.NotContains(t, output, "result: fail")

		var report openreportsv1alpha1.ClusterReport
		assert.NoError(t, yaml.Unmarshal(out.Bytes(), &report))
		assert.Equal(t, report.Summary.Warn, 1)
		assert.Equal(t, report.Summary.Fail, 0)
	})

	t.Run("without audit-warn, policy-report produces fail result and exits 1", func(t *testing.T) {
		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--policy-report"})
		err := cmd.Execute()
		assert.Error(t, err)

		output := out.String()
		assert.Contains(t, output, "kind: ClusterReport")
		assert.Contains(t, output, "result: fail")

		var report openreportsv1alpha1.ClusterReport
		assert.NoError(t, yaml.Unmarshal(out.Bytes(), &report))
		assert.Equal(t, report.Summary.Fail, 1)
		assert.Equal(t, report.Summary.Warn, 0)
	})

	t.Run("deny policy with audit-warn and policy-report produces fail result and exits 1", func(t *testing.T) {
		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		denyPolicyPath := "../../../../../test/cli/test-validating-policy/disallow-host-path/policy.yaml"
		denyResourcePath := "../../../../../test/cli/test-validating-policy/disallow-host-path/pod2.yaml"
		cmd.SetArgs([]string{denyPolicyPath, "--resource", denyResourcePath, "--audit-warn", "--policy-report"})
		err := cmd.Execute()
		assert.Error(t, err)

		output := out.String()
		assert.Contains(t, output, "kind: ClusterReport")
		assert.Contains(t, output, "result: fail")
		assert.NotContains(t, output, "result: warn")

		var report openreportsv1alpha1.ClusterReport
		assert.NoError(t, yaml.Unmarshal(out.Bytes(), &report))
		assert.Equal(t, report.Summary.Fail, 1)
		assert.Equal(t, report.Summary.Warn, 0)
	})

	t.Run("audit-only image validating policy with audit-warn and policy-report produces warn result and exit 0", func(t *testing.T) {
		tmpDir := t.TempDir()
		ivpolPath := filepath.Join(tmpDir, "ivpol-audit.yaml")
		resourcePath := filepath.Join(tmpDir, "pod.yaml")

		policyContent := `apiVersion: policies.kyverno.io/v1beta1
kind: ImageValidatingPolicy
metadata:
  name: ivpol-audit
spec:
  validationActions:
  - Audit
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      operations: ["CREATE"]
      resources: ["pods"]
  matchImageReferences:
  - glob: "registry.example.com/*"
  validationConfigurations:
    verifyDigest: false
    required: false
  attestors:
  - name: unused
    cosign:
      key:
        data: unused
  validations:
  - expression: "false"
    message: "offline audit failure"
`
		resourceContent := `apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: app
    image: "example.com/nginx:1.27"
`
		assert.NoError(t, os.WriteFile(ivpolPath, []byte(policyContent), 0o600))
		assert.NoError(t, os.WriteFile(resourcePath, []byte(resourceContent), 0o600))

		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{ivpolPath, "--resource", resourcePath, "--audit-warn", "--policy-report"})
		err := cmd.Execute()
		assert.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "kind: ClusterReport")
		assert.Contains(t, output, "result: warn")
		assert.NotContains(t, output, "result: fail")

		var report openreportsv1alpha1.ClusterReport
		assert.NoError(t, yaml.Unmarshal(out.Bytes(), &report))
		assert.Equal(t, report.Summary.Warn, 1)
		assert.Equal(t, report.Summary.Fail, 0)
	})

	t.Run("audit-warn in default output labels violations as warning", func(t *testing.T) {
		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--audit-warn", "--remove-color"})
		err := cmd.Execute()
		assert.NoError(t, err)

		output := out.String()
		assert.Contains(t, output, "warning:")
		assert.NotContains(t, output, "failed:")
		assert.Contains(t, output, "warn: 1")
		assert.Contains(t, output, "fail: 0")
	})

	t.Run("without audit-warn, default output labels violations as failed", func(t *testing.T) {
		cmd := Command()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--remove-color"})
		err := cmd.Execute()
		assert.Error(t, err)

		output := out.String()
		assert.Contains(t, output, "failed:")
		assert.Contains(t, output, "fail: 1")
		assert.Contains(t, output, "warn: 0")
	})
}

func TestApply_ImageValidatingPolicy_JSONPayload_PolicyException(t *testing.T) {
	const ivpol1YAML = `apiVersion: policies.kyverno.io/v1beta1
kind: ImageValidatingPolicy
metadata:
  name: ivpol-json-1
spec:
  evaluation:
    mode: JSON
  validationConfigurations:
    verifyDigest: false
    required: false
  matchImageReferences:
  - glob: "ghcr.io/*"
  images:
  - name: app
    expression: "[object.app]"
  attestors:
  - name: notary
    notary:
      certs:
        value: |-
          -----BEGIN CERTIFICATE-----
          MIIDTTCCAjWgAwIBAgIJAPI+zAzn4s0xMA0GCSqGSIb3DQEBCwUAMEwxCzAJBgNV
          BAYTAlVTMQswCQYDVQQIDAJXQTEQMA4GA1UEBwwHU2VhdHRsZTEPMA0GA1UECgwG
          Tm90YXJ5MQ0wCwYDVQQDDAR0ZXN0MB4XDTIzMDUyMjIxMTUxOFoXDTMzMDUxOTIx
          MTUxOFowTDELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAldBMRAwDgYDVQQHDAdTZWF0
          dGxlMQ8wDQYDVQQKDAZOb3RhcnkxDTALBgNVBAMMBHRlc3QwggEiMA0GCSqGSIb3
          DQEBAQUAA4IBDwAwggEKAoIBAQDNhTwv+QMk7jEHufFfIFlBjn2NiJaYPgL4eBS+
          b+o37ve5Zn9nzRppV6kGsa161r9s2KkLXmJrojNy6vo9a6g6RtZ3F6xKiWLUmbAL
          hVTCfYw/2n7xNlVMjyyUpE+7e193PF8HfQrfDFxe2JnX5LHtGe+X9vdvo2l41R6m
          Iia04DvpMdG4+da2tKPzXIuLUz/FDb6IODO3+qsqQLwEKmmUee+KX+3yw8I6G1y0
          Vp0mnHfsfutlHeG8gazCDlzEsuD4QJ9BKeRf2Vrb0ywqNLkGCbcCWF2H5Q80Iq/f
          ETVO9z88R7WheVdEjUB8UrY7ZMLdADM14IPhY2Y+tLaSzEVZAgMBAAGjMjAwMAkG
          A1UdEwQCMAAwDgYDVR0PAQH/BAQDAgeAMBMGA1UdJQQMMAoGCCsGAQUFBwMDMA0G
          CSqGSIb3DQEBCwUAA4IBAQBX7x4Ucre8AIUmXZ5PUK/zUBVOrZZzR1YE8w86J4X9
          kYeTtlijf9i2LTZMfGuG0dEVFN4ae3CCpBst+ilhIndnoxTyzP+sNy4RCRQ2Y/k8
          Zq235KIh7uucq96PL0qsF9s2RpTKXxyOGdtp9+HO0Ty5txJE2txtLDUIVPK5WNDF
          ByCEQNhtHgN6V20b8KU2oLBZ9vyB8V010dQz0NRTDLhkcvJig00535/LUylECYAJ
          5/jn6XKt6UYCQJbVNzBg/YPGc1RF4xdsGVDBben/JXpeGEmkdmXPILTKd9tZ5TC0
          uOKpF5rWAruB5PCIrquamOejpXV9aQA/K2JQDuc0mcKz
          -----END CERTIFICATE-----
  validations:
  - expression: "images.app.size() == 0"
    message: "no images allowed"
`

	const ivpol2YAML = `apiVersion: policies.kyverno.io/v1beta1
kind: ImageValidatingPolicy
metadata:
  name: ivpol-json-2
spec:
  evaluation:
    mode: JSON
  validationConfigurations:
    verifyDigest: false
    required: false
  matchImageReferences:
  - glob: "ghcr.io/*"
  images:
  - name: app
    expression: "[object.app]"
  attestors:
  - name: notary
    notary:
      certs:
        value: |-
          -----BEGIN CERTIFICATE-----
          MIIDTTCCAjWgAwIBAgIJAPI+zAzn4s0xMA0GCSqGSIb3DQEBCwUAMEwxCzAJBgNV
          BAYTAlVTMQswCQYDVQQIDAJXQTEQMA4GA1UEBwwHU2VhdHRsZTEPMA0GA1UECgwG
          Tm90YXJ5MQ0wCwYDVQQDDAR0ZXN0MB4XDTIzMDUyMjIxMTUxOFoXDTMzMDUxOTIx
          MTUxOFowTDELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAldBMRAwDgYDVQQHDAdTZWF0
          dGxlMQ8wDQYDVQQKDAZOb3RhcnkxDTALBgNVBAMMBHRlc3QwggEiMA0GCSqGSIb3
          DQEBAQUAA4IBDwAwggEKAoIBAQDNhTwv+QMk7jEHufFfIFlBjn2NiJaYPgL4eBS+
          b+o37ve5Zn9nzRppV6kGsa161r9s2KkLXmJrojNy6vo9a6g6RtZ3F6xKiWLUmbAL
          hVTCfYw/2n7xNlVMjyyUpE+7e193PF8HfQrfDFxe2JnX5LHtGe+X9vdvo2l41R6m
          Iia04DvpMdG4+da2tKPzXIuLUz/FDb6IODO3+qsqQLwEKmmUee+KX+3yw8I6G1y0
          Vp0mnHfsfutlHeG8gazCDlzEsuD4QJ9BKeRf2Vrb0ywqNLkGCbcCWF2H5Q80Iq/f
          ETVO9z88R7WheVdEjUB8UrY7ZMLdADM14IPhY2Y+tLaSzEVZAgMBAAGjMjAwMAkG
          A1UdEwQCMAAwDgYDVR0PAQH/BAQDAgeAMBMGA1UdJQQMMAoGCCsGAQUFBwMDMA0G
          CSqGSIb3DQEBCwUAA4IBAQBX7x4Ucre8AIUmXZ5PUK/zUBVOrZZzR1YE8w86J4X9
          kYeTtlijf9i2LTZMfGuG0dEVFN4ae3CCpBst+ilhIndnoxTyzP+sNy4RCRQ2Y/k8
          Zq235KIh7uucq96PL0qsF9s2RpTKXxyOGdtp9+HO0Ty5txJE2txtLDUIVPK5WNDF
          ByCEQNhtHgN6V20b8KU2oLBZ9vyB8V010dQz0NRTDLhkcvJig00535/LUylECYAJ
          5/jn6XKt6UYCQJbVNzBg/YPGc1RF4xdsGVDBben/JXpeGEmkdmXPILTKd9tZ5TC0
          uOKpF5rWAruB5PCIrquamOejpXV9aQA/K2JQDuc0mcKz
          -----END CERTIFICATE-----
  validations:
  - expression: "images.app.size() == 1"
    message: "must have 1 image"
`

	const matchingExYAML = `apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: ex-matching
  namespace: test-ns
spec:
  policyRefs:
  - name: ivpol-json-1
    kind: ImageValidatingPolicy
`

	const otherExYAML = `apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: ex-other
  namespace: test-ns
spec:
  policyRefs:
  - name: other-policy
    kind: ImageValidatingPolicy
`

	const secondMatchingExYAML = `apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: ex-second
  namespace: test-ns
spec:
  policyRefs:
  - name: ivpol-json-1
    kind: ImageValidatingPolicy
`

	const payloadJSON = `{"app": "ghcr.io/kyverno/test:1.0"}`

	const testNotaryCert = `          -----BEGIN CERTIFICATE-----
          MIIDTTCCAjWgAwIBAgIJAPI+zAzn4s0xMA0GCSqGSIb3DQEBCwUAMEwxCzAJBgNV
          BAYTAlVTMQswCQYDVQQIDAJXQTEQMA4GA1UEBwwHU2VhdHRsZTEPMA0GA1UECgwG
          Tm90YXJ5MQ0wCwYDVQQDDAR0ZXN0MB4XDTIzMDUyMjIxMTUxOFoXDTMzMDUxOTIx
          MTUxOFowTDELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAldBMRAwDgYDVQQHDAdTZWF0
          dGxlMQ8wDQYDVQQKDAZOb3RhcnkxDTALBgNVBAMMBHRlc3QwggEiMA0GCSqGSIb3
          DQEBAQUAA4IBDwAwggEKAoIBAQDNhTwv+QMk7jEHufFfIFlBjn2NiJaYPgL4eBS+
          b+o37ve5Zn9nzRppV6kGsa161r9s2KkLXmJrojNy6vo9a6g6RtZ3F6xKiWLUmbAL
          hVTCfYw/2n7xNlVMjyyUpE+7e193PF8HfQrfDFxe2JnX5LHtGe+X9vdvo2l41R6m
          Iia04DvpMdG4+da2tKPzXIuLUz/FDb6IODO3+qsqQLwEKmmUee+KX+3yw8I6G1y0
          Vp0mnHfsfutlHeG8gazCDlzEsuD4QJ9BKeRf2Vrb0ywqNLkGCbcCWF2H5Q80Iq/f
          ETVO9z88R7WheVdEjUB8UrY7ZMLdADM14IPhY2Y+tLaSzEVZAgMBAAGjMjAwMAkG
          A1UdEwQCMAAwDgYDVR0PAQH/BAQDAgeAMBMGA1UdJQQMMAoGCCsGAQUFBwMDMA0G
          CSqGSIb3DQEBCwUAA4IBAQBX7x4Ucre8AIUmXZ5PUK/zUBVOrZZzR1YE8w86J4X9
          kYeTtlijf9i2LTZMfGuG0dEVFN4ae3CCpBst+ilhIndnoxTyzP+sNy4RCRQ2Y/k8
          Zq235KIh7uucq96PL0qsF9s2RpTKXxyOGdtp9+HO0Ty5txJE2txtLDUIVPK5WNDF
          ByCEQNhtHgN6V20b8KU2oLBZ9vyB8V010dQz0NRTDLhkcvJig00535/LUylECYAJ
          5/jn6XKt6UYCQJbVNzBg/YPGc1RF4xdsGVDBben/JXpeGEmkdmXPILTKd9tZ5TC0
          uOKpF5rWAruB5PCIrquamOejpXV9aQA/K2JQDuc0mcKz
          -----END CERTIFICATE-----`

	dir := t.TempDir()
	ivpol1Path := filepath.Join(dir, "ivpol1.yaml")
	ivpol2Path := filepath.Join(dir, "ivpol2.yaml")
	matchingExPath := filepath.Join(dir, "ex-matching.yaml")
	otherExPath := filepath.Join(dir, "ex-other.yaml")
	secondMatchingExPath := filepath.Join(dir, "ex-second.yaml")
	jsonPath := filepath.Join(dir, "payload.json")

	require.NoError(t, os.WriteFile(ivpol1Path, []byte(ivpol1YAML), 0o600))
	require.NoError(t, os.WriteFile(ivpol2Path, []byte(ivpol2YAML), 0o600))
	require.NoError(t, os.WriteFile(matchingExPath, []byte(matchingExYAML), 0o600))
	require.NoError(t, os.WriteFile(otherExPath, []byte(otherExYAML), 0o600))
	require.NoError(t, os.WriteFile(secondMatchingExPath, []byte(secondMatchingExYAML), 0o600))
	require.NoError(t, os.WriteFile(jsonPath, []byte(payloadJSON), 0o600))

	t.Run("matching full-exemption exception produces a Skip policy response", func(t *testing.T) {
		config := &ApplyCommandConfig{
			PolicyPaths: []string{ivpol1Path},
			JSONPaths:   []string{jsonPath},
			Exception:   []string{matchingExPath},
		}
		rc, _, _, responses, err := config.applyCommandHelper(context.Background(), io.Discard)
		require.NoError(t, err)
		assert.Equal(t, 1, rc.Skip)
		assert.Equal(t, 0, rc.Fail)
		require.Len(t, responses, 1)
		require.Len(t, responses[0].PolicyResponse.Rules, 1)
		rule := responses[0].PolicyResponse.Rules[0]
		assert.Equal(t, engineapi.RuleStatusSkip, rule.Status())
		assert.Contains(t, rule.Message(), "test-ns/ex-matching")
	})

	t.Run("non-matching exception preserves normal evaluation", func(t *testing.T) {
		config := &ApplyCommandConfig{
			PolicyPaths: []string{ivpol1Path},
			JSONPaths:   []string{jsonPath},
			Exception:   []string{otherExPath},
		}
		rc, _, _, responses, err := config.applyCommandHelper(context.Background(), io.Discard)
		require.NoError(t, err)
		assert.Equal(t, 0, rc.Skip)
		assert.Equal(t, 1, rc.Fail)
		require.Len(t, responses, 1)
		require.Len(t, responses[0].PolicyResponse.Rules, 1)
		rule := responses[0].PolicyResponse.Rules[0]
		assert.Equal(t, engineapi.RuleStatusFail, rule.Status())
		assert.Contains(t, rule.Message(), "no images allowed")
	})

	t.Run("multiple matching exceptions are all present in Skip response", func(t *testing.T) {
		config := &ApplyCommandConfig{
			PolicyPaths: []string{ivpol1Path},
			JSONPaths:   []string{jsonPath},
			Exception:   []string{matchingExPath, secondMatchingExPath},
		}
		rc, _, _, responses, err := config.applyCommandHelper(context.Background(), io.Discard)
		require.NoError(t, err)
		assert.Equal(t, 1, rc.Skip)
		require.Len(t, responses, 1)
		require.Len(t, responses[0].PolicyResponse.Rules, 1)
		rule := responses[0].PolicyResponse.Rules[0]
		assert.Equal(t, engineapi.RuleStatusSkip, rule.Status())
		assert.Contains(t, rule.Message(), "test-ns/ex-matching")
		assert.Contains(t, rule.Message(), "test-ns/ex-second")
		assert.Len(t, rule.Exceptions(), 2)
	})

	t.Run("multiple IVPOLs retain their own responses without overwriting", func(t *testing.T) {
		config := &ApplyCommandConfig{
			PolicyPaths: []string{ivpol1Path, ivpol2Path},
			JSONPaths:   []string{jsonPath},
			Exception:   []string{matchingExPath},
		}
		rc, _, _, responses, err := config.applyCommandHelper(context.Background(), io.Discard)
		require.NoError(t, err)
		assert.Equal(t, 1, rc.Skip)
		assert.Equal(t, 1, rc.Pass)
		require.Len(t, responses, 2)
		byPolicy := make(map[string]engineapi.EngineResponse)
		for _, resp := range responses {
			byPolicy[resp.Policy().GetName()] = resp
		}
		require.Contains(t, byPolicy, "ivpol-json-1")
		require.Contains(t, byPolicy, "ivpol-json-2")
		require.Len(t, byPolicy["ivpol-json-1"].PolicyResponse.Rules, 1)
		require.Len(t, byPolicy["ivpol-json-2"].PolicyResponse.Rules, 1)
		assert.Equal(t, engineapi.RuleStatusSkip, byPolicy["ivpol-json-1"].PolicyResponse.Rules[0].Status())
		assert.Equal(t, engineapi.RuleStatusPass, byPolicy["ivpol-json-2"].PolicyResponse.Rules[0].Status())
	})

	t.Run("explain output displays SKIP verdict in trace output", func(t *testing.T) {
		var out bytes.Buffer
		config := &ApplyCommandConfig{
			PolicyPaths: []string{ivpol1Path},
			JSONPaths:   []string{jsonPath},
			Exception:   []string{matchingExPath},
			Explain:     true,
		}
		rc, _, _, responses, err := config.applyCommandHelper(context.Background(), &out)
		require.NoError(t, err)
		assert.Equal(t, 1, rc.Skip)
		require.Len(t, responses, 1)
		assert.Contains(t, out.String(), "SKIP")
	})

	t.Run("expired exception does not exempt payload and results in failure", func(t *testing.T) {
		expiredExPath := filepath.Join(dir, "ex-expired.yaml")
		expiredExYAML := `apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: ex-expired
  namespace: test-ns
spec:
  expiresAt: "2020-01-01T00:00:00Z"
  policyRefs:
  - name: ivpol-json-1
    kind: ImageValidatingPolicy
`
		require.NoError(t, os.WriteFile(expiredExPath, []byte(expiredExYAML), 0o600))
		config := &ApplyCommandConfig{
			PolicyPaths: []string{ivpol1Path},
			JSONPaths:   []string{jsonPath},
			Exception:   []string{expiredExPath},
		}
		rc, _, _, responses, err := config.applyCommandHelper(context.Background(), io.Discard)
		require.NoError(t, err)
		assert.Equal(t, 0, rc.Skip)
		assert.Equal(t, 1, rc.Fail)
		require.Len(t, responses, 1)
		assert.Equal(t, engineapi.RuleStatusFail, responses[0].PolicyResponse.Rules[0].Status())
	})

	t.Run("multiple NamespacedImageValidatingPolicies with identical names in different namespaces retain both responses", func(t *testing.T) {
		nivpol1YAML := `apiVersion: policies.kyverno.io/v1beta1
kind: NamespacedImageValidatingPolicy
metadata:
  name: nivpol-json-same-name
  namespace: tenant-a
spec:
  evaluation:
    mode: JSON
  validationConfigurations:
    verifyDigest: false
    required: false
  matchImageReferences:
  - glob: "ghcr.io/*"
  images:
  - name: app
    expression: "[object.app]"
  attestors:
  - name: notary
    notary:
      certs:
        value: |-
` + testNotaryCert + `
  validations:
  - expression: "images.app.size() == 0"
    message: "no images allowed"
`
		nivpol2YAML := `apiVersion: policies.kyverno.io/v1beta1
kind: NamespacedImageValidatingPolicy
metadata:
  name: nivpol-json-same-name
  namespace: tenant-b
spec:
  evaluation:
    mode: JSON
  validationConfigurations:
    verifyDigest: false
    required: false
  matchImageReferences:
  - glob: "ghcr.io/*"
  images:
  - name: app
    expression: "[object.app]"
  attestors:
  - name: notary
    notary:
      certs:
        value: |-
` + testNotaryCert + `
  validations:
  - expression: "images.app.size() == 1"
    message: "must have 1 image"
`
		nivpol1Path := filepath.Join(dir, "nivpol1.yaml")
		nivpol2Path := filepath.Join(dir, "nivpol2.yaml")
		require.NoError(t, os.WriteFile(nivpol1Path, []byte(nivpol1YAML), 0o600))
		require.NoError(t, os.WriteFile(nivpol2Path, []byte(nivpol2YAML), 0o600))

		config := &ApplyCommandConfig{
			PolicyPaths: []string{nivpol1Path, nivpol2Path},
			JSONPaths:   []string{jsonPath},
		}
		rc, _, _, responses, err := config.applyCommandHelper(context.Background(), io.Discard)
		require.NoError(t, err)
		assert.Equal(t, 1, rc.Pass)
		assert.Equal(t, 1, rc.Fail)
		require.Len(t, responses, 2)

		byKey := make(map[string]engineapi.EngineResponse)
		for _, resp := range responses {
			key := resp.Policy().GetNamespace() + "/" + resp.Policy().GetName()
			byKey[key] = resp
		}
		require.Contains(t, byKey, "tenant-a/nivpol-json-same-name")
		require.Contains(t, byKey, "tenant-b/nivpol-json-same-name")

		respA := byKey["tenant-a/nivpol-json-same-name"]
		require.Len(t, respA.PolicyResponse.Rules, 1)
		assert.Equal(t, engineapi.RuleStatusFail, respA.PolicyResponse.Rules[0].Status())

		respB := byKey["tenant-b/nivpol-json-same-name"]
		require.Len(t, respB.PolicyResponse.Rules, 1)
		assert.Equal(t, engineapi.RuleStatusPass, respB.PolicyResponse.Rules[0].Status())
	})

	t.Run("same-name cluster-scoped and namespaced policy exception isolation", func(t *testing.T) {
		const clusterIvpolYAML = `apiVersion: policies.kyverno.io/v1beta1
kind: ImageValidatingPolicy
metadata:
  name: ivpol-same-name
spec:
  evaluation:
    mode: JSON
  validationConfigurations:
    verifyDigest: false
    required: false
  matchImageReferences:
  - glob: "ghcr.io/*"
  images:
  - name: app
    expression: "[object.app]"
  attestors:
  - name: notary
    notary:
      certs:
        value: |-
` + testNotaryCert + `
  validations:
  - expression: "images.app.size() == 0"
    message: "no images allowed"
`
		const namespacedIvpolYAML = `apiVersion: policies.kyverno.io/v1beta1
kind: NamespacedImageValidatingPolicy
metadata:
  name: ivpol-same-name
  namespace: tenant-a
spec:
  evaluation:
    mode: JSON
  validationConfigurations:
    verifyDigest: false
    required: false
  matchImageReferences:
  - glob: "ghcr.io/*"
  images:
  - name: app
    expression: "[object.app]"
  attestors:
  - name: notary
    notary:
      certs:
        value: |-
` + testNotaryCert + `
  validations:
  - expression: "images.app.size() == 0"
    message: "no images allowed"
`
		const polexClusterYAML = `apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: ex-cluster-same-name
  namespace: test-ns
spec:
  policyRefs:
  - name: ivpol-same-name
    kind: ImageValidatingPolicy
`
		dir := t.TempDir()
		clusterPolicyPath := filepath.Join(dir, "cluster-policy.yaml")
		namespacedPolicyPath := filepath.Join(dir, "namespaced-policy.yaml")
		polexPath := filepath.Join(dir, "polex.yaml")
		jsonPath := filepath.Join(dir, "payload.json")

		require.NoError(t, os.WriteFile(clusterPolicyPath, []byte(clusterIvpolYAML), 0o600))
		require.NoError(t, os.WriteFile(namespacedPolicyPath, []byte(namespacedIvpolYAML), 0o600))
		require.NoError(t, os.WriteFile(polexPath, []byte(polexClusterYAML), 0o600))
		require.NoError(t, os.WriteFile(jsonPath, []byte(payloadJSON), 0o600))

		config := &ApplyCommandConfig{
			PolicyPaths: []string{clusterPolicyPath, namespacedPolicyPath},
			JSONPaths:   []string{jsonPath},
			Exception:   []string{polexPath},
		}
		rc, _, _, responses, err := config.applyCommandHelper(context.Background(), io.Discard)
		require.NoError(t, err)
		assert.Equal(t, 1, rc.Skip)
		assert.Equal(t, 1, rc.Fail)
		assert.Equal(t, 0, rc.Pass)
		require.Len(t, responses, 2)

		var clusterResp, namespacedResp *engineapi.EngineResponse
		for i := range responses {
			p := responses[i].Policy()
			if p.GetKind() == "ImageValidatingPolicy" && p.GetName() == "ivpol-same-name" && p.GetNamespace() == "" {
				clusterResp = &responses[i]
			} else if p.GetKind() == "NamespacedImageValidatingPolicy" && p.GetName() == "ivpol-same-name" && p.GetNamespace() == "tenant-a" {
				namespacedResp = &responses[i]
			}
		}
		require.NotNil(t, clusterResp, "cluster-scoped policy response must be present")
		require.NotNil(t, namespacedResp, "namespaced policy response must be present")

		require.Len(t, clusterResp.PolicyResponse.Rules, 1)
		clusterRule := clusterResp.PolicyResponse.Rules[0]
		assert.Equal(t, engineapi.RuleStatusSkip, clusterRule.Status())
		assert.Contains(t, clusterRule.Message(), "test-ns/ex-cluster-same-name")
		require.Len(t, clusterRule.Exceptions(), 1)
		assert.Equal(t, "ex-cluster-same-name", clusterRule.Exceptions()[0].GetName())
		assert.Equal(t, "test-ns", clusterRule.Exceptions()[0].GetNamespace())

		require.Len(t, namespacedResp.PolicyResponse.Rules, 1)
		namespacedRule := namespacedResp.PolicyResponse.Rules[0]
		assert.Equal(t, engineapi.RuleStatusFail, namespacedRule.Status())
		assert.Empty(t, namespacedRule.Exceptions())
	})

	t.Run("namespaced PolicyException targeting exempts matching policy and retains non-exempt policy", func(t *testing.T) {
		const nivpolExemptYAML = `apiVersion: policies.kyverno.io/v1beta1
kind: NamespacedImageValidatingPolicy
metadata:
  name: nivpol-exempt
  namespace: tenant-a
spec:
  evaluation:
    mode: JSON
  validationConfigurations:
    verifyDigest: false
    required: false
  matchImageReferences:
  - glob: "ghcr.io/*"
  images:
  - name: app
    expression: "[object.app]"
  attestors:
  - name: notary
    notary:
      certs:
        value: |-
` + testNotaryCert + `
  validations:
  - expression: "images.app.size() == 0"
    message: "no images allowed"
`
		const nivpolEnforcedYAML = `apiVersion: policies.kyverno.io/v1beta1
kind: NamespacedImageValidatingPolicy
metadata:
  name: nivpol-enforced
  namespace: tenant-b
spec:
  evaluation:
    mode: JSON
  validationConfigurations:
    verifyDigest: false
    required: false
  matchImageReferences:
  - glob: "ghcr.io/*"
  images:
  - name: app
    expression: "[object.app]"
  attestors:
  - name: notary
    notary:
      certs:
        value: |-
` + testNotaryCert + `
  validations:
  - expression: "images.app.size() == 0"
    message: "no images allowed"
`
		const polexNivpolYAML = `apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: ex-nivpol-exempt
  namespace: tenant-a
spec:
  policyRefs:
  - name: nivpol-exempt
    kind: NamespacedImageValidatingPolicy
`
		dir := t.TempDir()
		exemptPolicyPath := filepath.Join(dir, "exempt-policy.yaml")
		enforcedPolicyPath := filepath.Join(dir, "enforced-policy.yaml")
		polexPath := filepath.Join(dir, "polex.yaml")
		jsonPath := filepath.Join(dir, "payload.json")

		require.NoError(t, os.WriteFile(exemptPolicyPath, []byte(nivpolExemptYAML), 0o600))
		require.NoError(t, os.WriteFile(enforcedPolicyPath, []byte(nivpolEnforcedYAML), 0o600))
		require.NoError(t, os.WriteFile(polexPath, []byte(polexNivpolYAML), 0o600))
		require.NoError(t, os.WriteFile(jsonPath, []byte(payloadJSON), 0o600))

		config := &ApplyCommandConfig{
			PolicyPaths: []string{exemptPolicyPath, enforcedPolicyPath},
			JSONPaths:   []string{jsonPath},
			Exception:   []string{polexPath},
		}
		rc, _, _, responses, err := config.applyCommandHelper(context.Background(), io.Discard)
		require.NoError(t, err)
		assert.Equal(t, 1, rc.Skip)
		assert.Equal(t, 1, rc.Fail)
		assert.Equal(t, 0, rc.Pass)
		require.Len(t, responses, 2)

		var exemptResp, enforcedResp *engineapi.EngineResponse
		for i := range responses {
			p := responses[i].Policy()
			if p.GetName() == "nivpol-exempt" && p.GetNamespace() == "tenant-a" {
				exemptResp = &responses[i]
			} else if p.GetName() == "nivpol-enforced" && p.GetNamespace() == "tenant-b" {
				enforcedResp = &responses[i]
			}
		}
		require.NotNil(t, exemptResp, "exempt policy response must be present")
		require.NotNil(t, enforcedResp, "enforced policy response must be present")

		assert.Equal(t, "NamespacedImageValidatingPolicy", exemptResp.Policy().GetKind())
		require.Len(t, exemptResp.PolicyResponse.Rules, 1)
		exemptRule := exemptResp.PolicyResponse.Rules[0]
		assert.Equal(t, engineapi.RuleStatusSkip, exemptRule.Status())
		assert.Contains(t, exemptRule.Message(), "tenant-a/ex-nivpol-exempt")
		require.Len(t, exemptRule.Exceptions(), 1)
		assert.Equal(t, "ex-nivpol-exempt", exemptRule.Exceptions()[0].GetName())
		assert.Equal(t, "tenant-a", exemptRule.Exceptions()[0].GetNamespace())

		assert.Equal(t, "NamespacedImageValidatingPolicy", enforcedResp.Policy().GetKind())
		require.Len(t, enforcedResp.PolicyResponse.Rules, 1)
		enforcedRule := enforcedResp.PolicyResponse.Rules[0]
		assert.Equal(t, engineapi.RuleStatusFail, enforcedRule.Status())
		assert.Empty(t, enforcedRule.Exceptions())
	})
}
