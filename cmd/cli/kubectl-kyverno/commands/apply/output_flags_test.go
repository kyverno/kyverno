package apply

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestRejectFormatKeywordsAsOutputPath(t *testing.T) {
	tempDir := t.TempDir()
	origWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer func() {
		_ = os.Chdir(origWd)
	}()

	policyPath := filepath.Join(tempDir, "policy.yaml")
	resourcePath := filepath.Join(tempDir, "resource.yaml")
	require.NoError(t, os.WriteFile(policyPath, []byte(fmt.Sprintf(outputTestMutatingPolicy, `"team-a"`)), 0o600))
	require.NoError(t, os.WriteFile(resourcePath, []byte(outputTestPod), 0o600))

	testCases := []struct {
		keyword     string
		expectedErr string
	}{
		{keyword: "json", expectedErr: `invalid path "json": did you mean --output-format json? Use --output-dir to specify a file or directory for mutated resources`},
		{keyword: "yaml", expectedErr: `invalid path "yaml": did you mean --output-format yaml? Use --output-dir to specify a file or directory for mutated resources`},
		{keyword: "yml", expectedErr: `invalid path "yml": did you mean --output-format yaml? Use --output-dir to specify a file or directory for mutated resources`},
		{keyword: "table", expectedErr: `invalid path "table": did you mean --output-format table? Use --output-dir to specify a file or directory for mutated resources`},
		{keyword: "markdown", expectedErr: `invalid path "markdown": use --output-format for report output, or --output-dir to specify a file or directory for mutated resources`},
		{keyword: "junit", expectedErr: `invalid path "junit": use --output-format for report output, or --output-dir to specify a file or directory for mutated resources`},
	}

	for _, tc := range testCases {
		t.Run("flag -o "+tc.keyword, func(t *testing.T) {
			cmd := Command()
			out := &bytes.Buffer{}
			cmd.SetOut(out)
			cmd.SetErr(out)
			cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "-o", tc.keyword})

			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)

			// Crucial check: verify no directory or file named keyword was created
			_, statErr := os.Stat(filepath.Join(tempDir, tc.keyword))
			assert.True(t, os.IsNotExist(statErr), "file or directory %q should not have been created on disk", tc.keyword)
		})

		t.Run("flag --output-dir "+tc.keyword, func(t *testing.T) {
			cmd := Command()
			out := &bytes.Buffer{}
			cmd.SetOut(out)
			cmd.SetErr(out)
			cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--output-dir", tc.keyword})

			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)

			_, statErr := os.Stat(filepath.Join(tempDir, tc.keyword))
			assert.True(t, os.IsNotExist(statErr), "file or directory %q should not have been created on disk", tc.keyword)
		})
	}
}

func TestOutputDirMutatedPolicy(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	resourcePath := filepath.Join(dir, "resource.yaml")
	outputPath := filepath.Join(dir, "mutated.yaml")
	require.NoError(t, os.WriteFile(policyPath, []byte(fmt.Sprintf(outputTestMutatingPolicy, `"team-b"`)), 0o600))
	require.NoError(t, os.WriteFile(resourcePath, []byte(outputTestPod), 0o600))

	cmd := Command()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--output-dir", outputPath})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, out.String(), "pass: 1, fail: 0, warn: 0, error: 0, skip: 0")
	written, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Contains(t, string(written), "kind: Pod")
	assert.Contains(t, string(written), "team: team-b")
}

func TestOutputFormatTable(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	resourcePath := filepath.Join(dir, "resource.yaml")
	require.NoError(t, os.WriteFile(policyPath, []byte(fmt.Sprintf(outputTestMutatingPolicy, `"team-c"`)), 0o600))
	require.NoError(t, os.WriteFile(resourcePath, []byte(outputTestPod), 0o600))

	for _, flagArgs := range [][]string{
		{"--output-format", "table"},
		{"-t"},
		{"--table"},
	} {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		args := append([]string{policyPath, "--resource", resourcePath}, flagArgs...)
		cmd.SetArgs(args)
		err := cmd.Execute()
		require.NoError(t, err)

		outputStr := out.String()
		assert.Contains(t, outputStr, "POLICY", "table header should be printed")
		assert.Contains(t, outputStr, "RESOURCE", "table header should be printed")
		assert.Contains(t, outputStr, "RESULT", "table header should be printed")
		assert.NotContains(t, outputStr, "Mutation has been applied successfully")
		assert.NotContains(t, outputStr, "kind: Pod")
		// The "Applying N policy rule(s)..." banner must be suppressed in table
		// mode so the table is the sole content on stdout.
		assert.NotContains(t, outputStr, "Applying ", "banner must not appear in table output")
	}
}

func TestOutputFormatConflicts(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	resourcePath := filepath.Join(dir, "resource.yaml")
	require.NoError(t, os.WriteFile(policyPath, []byte(fmt.Sprintf(outputTestMutatingPolicy, `"team-d"`)), 0o600))
	require.NoError(t, os.WriteFile(resourcePath, []byte(outputTestPod), 0o600))

	t.Run("table output with policy report flag -p -t", func(t *testing.T) {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "-p", "-t"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table output format cannot be used with -p/--policy-report")
	})

	t.Run("table output with policy report flag -p --output-format table", func(t *testing.T) {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "-p", "--output-format", "table"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table output format cannot be used with -p/--policy-report")
	})

	t.Run("table output with generate-exceptions", func(t *testing.T) {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--generate-exceptions", "--output-format", "table"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table output format cannot be used with --generate-exceptions")
	})

	t.Run("conflicting --table and --output-format json", func(t *testing.T) {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--table", "--output-format", "json"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflicting output formats: --table and --output-format json cannot be used together")
	})

	t.Run("invalid output-format", func(t *testing.T) {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--output-format", "unsupported"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid output format "unsupported" (valid formats: table, json, yaml)`)
	})

	t.Run("conflicting --output and --output-dir flags", func(t *testing.T) {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--output", filepath.Join(dir, "dir1.yaml"), "--output-dir", filepath.Join(dir, "dir2.yaml")})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot use both --output and --output-dir")
	})

	t.Run("table output with stdin", func(t *testing.T) {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "-i", "-t"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "table output format cannot be used with -i/--stdin")
	})

	t.Run("policy report with generate exceptions", func(t *testing.T) {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "-p", "--generate-exceptions"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "-p/--policy-report and --generate-exceptions are mutually exclusive")
	})

	t.Run("explain with table output-format succeeds", func(t *testing.T) {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--explain", "--output-format", "table"})
		err := cmd.Execute()
		require.NoError(t, err)
	})

	t.Run("explain with -t flag succeeds", func(t *testing.T) {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--explain", "-t"})
		err := cmd.Execute()
		require.NoError(t, err)
	})
}

func TestOutputFormatWarningWithoutReport(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	resourcePath := filepath.Join(dir, "resource.yaml")
	require.NoError(t, os.WriteFile(policyPath, []byte(fmt.Sprintf(outputTestMutatingPolicy, `"team-e"`)), 0o600))
	require.NoError(t, os.WriteFile(resourcePath, []byte(outputTestPod), 0o600))

	cmd := Command()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "--output-format", "json"})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, stderr.String(), "WARNING: --output-format json has no effect without -p/--policy-report or --generate-exceptions")
	assert.Contains(t, stdout.String(), "pass: 1, fail: 0, warn: 0, error: 0, skip: 0")
}

const outputTestValidatingPolicy = `
apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: check-pod
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [CREATE]
      resources: [pods]
  validations:
  - expression: "object.metadata.name == 'pod'"
`

const outputTestFailingPod = `
apiVersion: v1
kind: Pod
metadata:
  name: not-the-pod
  namespace: default
  annotations: {}
spec:
  containers:
  - name: c
    image: nginx:latest
`

func TestPolicyReportOutputFormatSerialization(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	resourcePath := filepath.Join(dir, "resource.yaml")
	require.NoError(t, os.WriteFile(policyPath, []byte(outputTestValidatingPolicy), 0o600))
	require.NoError(t, os.WriteFile(resourcePath, []byte(outputTestPod), 0o600))

	t.Run("policy report in json format", func(t *testing.T) {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "-p", "--output-format", "json"})
		err := cmd.Execute()
		require.NoError(t, err)

		var parsed map[string]interface{}
		err = json.Unmarshal(out.Bytes(), &parsed)
		require.NoError(t, err, "output should be valid JSON: %s", out.String())
		assert.NotEmpty(t, parsed["kind"])
		assert.Equal(t, "openreports.io/v1alpha1", parsed["apiVersion"])
	})

	t.Run("policy report in json format with failing policy result", func(t *testing.T) {
		failingResourcePath := filepath.Join(dir, "failing_resource.yaml")
		require.NoError(t, os.WriteFile(failingResourcePath, []byte(outputTestFailingPod), 0o600))

		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", failingResourcePath, "-p", "--output-format", "json"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exit as there are policy violations")

		var parsed map[string]interface{}
		err = json.Unmarshal(out.Bytes(), &parsed)
		require.NoError(t, err, "output should be valid JSON: %s", out.String())
		assert.NotEmpty(t, parsed["kind"])
		assert.Equal(t, "openreports.io/v1alpha1", parsed["apiVersion"])

		// Assert summary has 1 failure
		summary, ok := parsed["summary"].(map[string]interface{})
		require.True(t, ok, "summary should be a map")
		assert.Equal(t, float64(1), summary["fail"])

		// Assert results contain the failed policy result
		results, ok := parsed["results"].([]interface{})
		require.True(t, ok, "results should be a list")
		require.NotEmpty(t, results)
		firstResult, ok := results[0].(map[string]interface{})
		require.True(t, ok, "result item should be a map")
		assert.Equal(t, "fail", firstResult["result"])
		assert.Equal(t, "check-pod", firstResult["policy"])
	})

	t.Run("policy report in yaml format", func(t *testing.T) {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "-p", "--output-format", "yaml"})
		err := cmd.Execute()
		require.NoError(t, err)

		var parsed map[string]interface{}
		err = yaml.Unmarshal(out.Bytes(), &parsed)
		require.NoError(t, err, "output should be valid YAML: %s", out.String())
		assert.NotEmpty(t, parsed["kind"])
		assert.Equal(t, "openreports.io/v1alpha1", parsed["apiVersion"])
	})

	t.Run("policy report defaults to yaml format when output-format is omitted", func(t *testing.T) {
		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "-p"})
		err := cmd.Execute()
		require.NoError(t, err)

		var parsed map[string]interface{}
		err = yaml.Unmarshal(out.Bytes(), &parsed)
		require.NoError(t, err, "output should be valid YAML: %s", out.String())
		assert.NotEmpty(t, parsed["kind"])
		assert.Equal(t, "openreports.io/v1alpha1", parsed["apiVersion"])
	})

	t.Run("namespaced policy report in json format has no YAML separator", func(t *testing.T) {
		const namespacedPolicy = `
apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: check-pod-ns
  namespace: test-ns
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [CREATE]
      resources: [pods]
  validations:
  - expression: "object.metadata.name == 'pod'"
`
		nsPolicyPath := filepath.Join(dir, "ns_policy.yaml")
		require.NoError(t, os.WriteFile(nsPolicyPath, []byte(namespacedPolicy), 0o600))

		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{nsPolicyPath, "--resource", resourcePath, "-p", "--output-format", "json"})
		err := cmd.Execute()
		require.NoError(t, err)

		outStr := out.String()
		assert.NotContains(t, outStr, "---", "JSON policy report output must not contain YAML document separator")
		var parsed map[string]interface{}
		err = json.Unmarshal(out.Bytes(), &parsed)
		require.NoError(t, err, "output should be valid JSON: %s", outStr)
		assert.Equal(t, "Report", parsed["kind"])
		assert.Equal(t, "openreports.io/v1alpha1", parsed["apiVersion"])
	})

	t.Run("multi-report in json format is wrapped in a list", func(t *testing.T) {
		const namespacedPolicy2 = `
apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: check-pod-ns-2
  namespace: test-ns-2
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [CREATE]
      resources: [pods]
  validations:
  - expression: "object.metadata.name == 'pod'"
`
		nsPolicy2Path := filepath.Join(dir, "ns_policy_2.yaml")
		require.NoError(t, os.WriteFile(nsPolicy2Path, []byte(namespacedPolicy2), 0o600))

		cmd := Command()
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetErr(out)
		cmd.SetArgs([]string{policyPath, nsPolicy2Path, "--resource", resourcePath, "-p", "--output-format", "json"})
		err := cmd.Execute()
		require.NoError(t, err)

		outStr := out.String()
		assert.NotContains(t, outStr, "---", "JSON policy report output must not contain YAML document separator")
		var parsed []map[string]interface{}
		err = json.Unmarshal(out.Bytes(), &parsed)
		require.NoError(t, err, "multi-report output should be valid JSON list: %s", outStr)
		require.Len(t, parsed, 2)
	})
}

func TestGenerateExceptionsJSONFormat(t *testing.T) {
	const (
		policy   = "../../../../../test/cli/test-validating-policy/check-deployment-labels/policy.yaml"
		resource = "../../../../../test/cli/test-validating-policy/check-deployment-labels/deployment1.yaml"
	)
	cmd := Command()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{policy, "--resource", resource, "--generate-exceptions", "--output-format", "json"})
	err := cmd.Execute()
	require.NoError(t, err)

	outStr := stdout.String()
	assert.NotContains(t, outStr, "---", "JSON exceptions output must not contain YAML document separator")
	for _, line := range strings.Split(strings.TrimSpace(outStr), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var parsed map[string]interface{}
		err := json.Unmarshal([]byte(line), &parsed)
		require.NoError(t, err, "each exception should be valid JSON: %s", line)
		assert.Equal(t, "PolicyException", parsed["kind"])
	}
}

func TestStructuredOutputDiagnosticsRouteToStderr(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	resourcePath := filepath.Join(dir, "resource.yaml")
	require.NoError(t, os.WriteFile(policyPath, []byte(outputTestValidatingPolicy), 0o600))
	require.NoError(t, os.WriteFile(resourcePath, []byte(outputTestPod), 0o600))

	cmd := Command()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{policyPath, "--resource", resourcePath, "-p", "--output-format", "json"})
	err := cmd.Execute()
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(stdout.Bytes(), &parsed)
	require.NoError(t, err, "stdout should be strictly valid JSON: %s", stdout.String())

	// Test exit diagnostic leak: warn-no-pass messages must route to stderr
	cmdWarn := Command()
	stdoutWarn := &bytes.Buffer{}
	stderrWarn := &bytes.Buffer{}
	cmdWarn.SetOut(stdoutWarn)
	cmdWarn.SetErr(stderrWarn)
	failingResourcePath := filepath.Join(dir, "failing_resource.yaml")
	require.NoError(t, os.WriteFile(failingResourcePath, []byte(outputTestFailingPod), 0o600))
	cmdWarn.SetArgs([]string{policyPath, "--resource", failingResourcePath, "-p", "--output-format", "json", "--warn-no-pass"})
	_ = cmdWarn.Execute()
	assert.NotContains(t, stdoutWarn.String(), "exit as no objects satisfied policy")
}
