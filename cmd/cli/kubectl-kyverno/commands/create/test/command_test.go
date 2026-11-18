package test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/deprecations"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommand(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	err := cmd.Execute()
	assert.NoError(t, err)
}

func TestCommandWithArgs(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{"foo"})
	err := cmd.Execute()
	assert.Error(t, err)
}

func TestCommandWithInvalidArg(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	cmd.SetArgs([]string{"foo"})
	err := cmd.Execute()
	assert.Error(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	expected := `Error: unknown command "foo" for "test"`
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

func TestParseResult(t *testing.T) {
	// 5 parts: without patched resource
	res := parseResult("policy,rule,resource,ns,Pod", "pass")
	assert.NotNil(t, res)
	assert.Equal(t, "policy", res.Policy)
	assert.Equal(t, "rule", res.Rule)
	assert.Equal(t, "resource", res.Resource)
	assert.Equal(t, "ns", res.Namespace)
	assert.Equal(t, "Pod", res.Kind)
	assert.Empty(t, res.PatchedResource)
	assert.Equal(t, "pass", res.Result)

	// 6 parts: with patched resource
	resMut := parseResult("policy,rule,resource,ns,Pod,patched.yaml", "pass")
	assert.NotNil(t, resMut)
	assert.Equal(t, "policy", resMut.Policy)
	assert.Equal(t, "rule", resMut.Rule)
	assert.Equal(t, "resource", resMut.Resource)
	assert.Equal(t, "ns", resMut.Namespace)
	assert.Equal(t, "Pod", resMut.Kind)
	assert.Equal(t, "patched.yaml", resMut.PatchedResource)
	assert.Equal(t, "pass", resMut.Result)

	// empty rule is allowed for ruleless policies (e.g. ValidatingPolicy)
	resRuleless := parseResult("policy,,resource,ns,Pod", "pass")
	assert.NotNil(t, resRuleless)
	assert.Equal(t, "policy", resRuleless.Policy)
	assert.Empty(t, resRuleless.Rule)
	assert.Equal(t, "resource", resRuleless.Resource)
	assert.Equal(t, "ns", resRuleless.Namespace)
	assert.Equal(t, "Pod", resRuleless.Kind)
	assert.Empty(t, resRuleless.PatchedResource)
	assert.Equal(t, "pass", resRuleless.Result)

	// empty policy, resource, or kind should be rejected
	assert.Nil(t, parseResult(",rule,resource,ns,Pod", "pass"))
	assert.Nil(t, parseResult("policy,rule,,ns,Pod", "pass"))
	assert.Nil(t, parseResult("policy,rule,resource,ns,", "pass"))
	assert.Nil(t, parseResult(",rule,resource,ns,Pod,patched.yaml", "pass"))

	// invalid counts
	assert.Nil(t, parseResult("foo,bar", "pass"))
	assert.Nil(t, parseResult("1,2,3,4,5,6,7", "pass"))
}

func TestCommandWithInvalidEntry(t *testing.T) {
	tests := []struct {
		name  string
		flag  string
		entry string
	}{
		{"invalid pass count", "--pass", "invalid,entry"},
		{"invalid fail count", "--fail", "invalid,entry"},
		{"invalid warn count", "--warn", "invalid,entry"},
		{"invalid error count", "--error", "invalid,entry"},
		{"invalid skip count", "--skip", "invalid,entry"},
		{"empty policy pass", "--pass", ",rule,resource,default,Pod"},
		{"empty resource pass", "--pass", "policy,rule,,default,Pod"},
		{"empty kind pass", "--pass", "policy,rule,resource,default,"},
		{"empty policy warn", "--warn", ",rule,resource,default,Pod"},
		{"empty policy error", "--error", ",rule,resource,default,Pod"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := Command()
			cmd.SetArgs([]string{tc.flag, tc.entry})
			err := cmd.Execute()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "expected policy,rule,resource,namespace,kind[,patchedResource]")
		})
	}
}

func TestCommandWithResults(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetArgs([]string{
		"-n", "my-test",
		"-p", "policy.yaml",
		"-r", "resource.yaml",
		"-f", "values.yaml",
		"--pass", "pol1,rule1,res1,default,Pod",
		"--pass", "pol2,rule2,res2,,ClusterRole,patched-resource.yaml",
		"--fail", "pol3,rule3,res3,test-ns,Pod",
		"--warn", "pol4,rule4,res4,default,Pod",
		"--error", "pol5,rule5,res5,default,Pod",
		"--skip", "pol6,rule6,res6,default,Pod",
		"--pass", "pol-ruleless,,res-cel,default,Pod",
	})
	err := cmd.Execute()
	assert.NoError(t, err)

	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	outputStr := strings.ReplaceAll(string(out), "\r\n", "\n")

	assert.Contains(t, outputStr, "apiVersion: cli.kyverno.io/v1alpha1")
	assert.Contains(t, outputStr, "kind: Test")
	assert.Contains(t, outputStr, `name: "my-test"`)
	assert.Contains(t, outputStr, "- policy.yaml")
	assert.Contains(t, outputStr, "- resource.yaml")
	assert.Contains(t, outputStr, "variables: values.yaml")

	// Verify namespaced resource renders quoted namespace/name
	assert.Contains(t, outputStr, `resources:
      - "default/res1"`)

	// Verify cluster-scoped resource without namespace renders quoted name
	assert.Contains(t, outputStr, `resources:
      - "res2"`)

	// Verify standard pass result does not include patchedResources
	if start := strings.Index(outputStr, "policy: pol1"); assert.GreaterOrEqual(t, start, 0) {
		entry := outputStr[start:]
		if end := strings.Index(entry, "\n  - policy:"); end >= 0 {
			entry = entry[:end]
		}
		assert.NotContains(t, entry, "patchedResources")
	}

	// Verify 6-part result renders quoted patchedResources
	if start := strings.Index(outputStr, "policy: pol2"); assert.GreaterOrEqual(t, start, 0) {
		entry := outputStr[start:]
		if end := strings.Index(entry, "\n  - policy:"); end >= 0 {
			entry = entry[:end]
		}
		assert.Contains(t, entry, `patchedResources: "patched-resource.yaml"`)
	}

	// Verify ruleless policy result omits rule field
	if start := strings.Index(outputStr, "policy: pol-ruleless"); assert.GreaterOrEqual(t, start, 0) {
		entry := outputStr[start:]
		if end := strings.Index(entry, "\n  - policy:"); end >= 0 {
			entry = entry[:end]
		}
		assert.NotContains(t, entry, "rule:")
	}

	assertResult := func(policy, expectedResult string) {
		for _, entry := range strings.Split(outputStr, "policy: ") {
			if strings.HasPrefix(entry, policy+"\n") {
				assert.Contains(t, entry, "result: "+expectedResult)
				return
			}
		}
		assert.Failf(t, "missing policy entry", "policy %s", policy)
	}

	assertResult("pol3", "fail")
	assertResult("pol4", "warn")
	assertResult("pol5", "error")
	assertResult("pol6", "skip")

	// Verify the generated test manifest with real test loader and deprecation checker
	testFile := filepath.Join(t.TempDir(), "kyverno-test.yaml")
	err = os.WriteFile(testFile, out, 0600)
	assert.NoError(t, err)

	testCases := test.LoadTest(nil, testFile)
	require.Len(t, testCases, 1)
	require.NoError(t, testCases[0].Err)
	parsedTest := testCases[0].Test
	require.NotNil(t, parsedTest)

	assert.False(t, deprecations.CheckTest(nil, testFile, parsedTest))
	assert.Equal(t, "cli.kyverno.io/v1alpha1", parsedTest.APIVersion)
	assert.Equal(t, "Test", parsedTest.Kind)
	assert.Equal(t, "my-test", parsedTest.ObjectMeta.Name)
	assert.Equal(t, []string{"policy.yaml"}, parsedTest.Policies)
	assert.Equal(t, []string{"resource.yaml"}, parsedTest.Resources)
	assert.Equal(t, "values.yaml", parsedTest.Variables)
	assert.Len(t, parsedTest.Results, 7)

	assert.Equal(t, "pol1", parsedTest.Results[0].Policy)
	assert.Equal(t, []string{"default/res1"}, parsedTest.Results[0].Resources)
	assert.Empty(t, parsedTest.Results[0].PatchedResources)
	assert.Equal(t, "pass", string(parsedTest.Results[0].Result))

	assert.Equal(t, "pol2", parsedTest.Results[1].Policy)
	assert.Equal(t, []string{"res2"}, parsedTest.Results[1].Resources)
	assert.Equal(t, "patched-resource.yaml", parsedTest.Results[1].PatchedResources)

	assert.Equal(t, "pol-ruleless", parsedTest.Results[2].Policy)
	assert.Empty(t, parsedTest.Results[2].Rule)
	assert.Equal(t, []string{"default/res-cel"}, parsedTest.Results[2].Resources)
	assert.Equal(t, "pass", string(parsedTest.Results[2].Result))

	assert.Equal(t, "pol3", parsedTest.Results[3].Policy)
	assert.Equal(t, []string{"test-ns/res3"}, parsedTest.Results[3].Resources)
	assert.Equal(t, "fail", string(parsedTest.Results[3].Result))

	assert.Equal(t, "pol4", parsedTest.Results[4].Policy)
	assert.Equal(t, "warn", string(parsedTest.Results[4].Result))

	assert.Equal(t, "pol5", parsedTest.Results[5].Policy)
	assert.Equal(t, "error", string(parsedTest.Results[5].Result))

	assert.Equal(t, "pol6", parsedTest.Results[6].Policy)
	assert.Equal(t, "skip", string(parsedTest.Results[6].Result))
}

func TestCommandWithNumericAndBooleanName(t *testing.T) {
	for _, name := range []string{"123", "true", "007"} {
		t.Run("name="+name, func(t *testing.T) {
			cmd := Command()
			b := bytes.NewBufferString("")
			cmd.SetOut(b)
			cmd.SetArgs([]string{
				"-n", name,
				"--pass", "pol1,rule1,res1,default,Pod",
			})
			err := cmd.Execute()
			require.NoError(t, err)

			testFile := filepath.Join(t.TempDir(), "kyverno-test.yaml")
			err = os.WriteFile(testFile, b.Bytes(), 0600)
			require.NoError(t, err)

			testCases := test.LoadTest(nil, testFile)
			require.Len(t, testCases, 1)
			require.NoError(t, testCases[0].Err)
			require.NotNil(t, testCases[0].Test)
			assert.Equal(t, name, testCases[0].Test.ObjectMeta.Name)
		})
	}
}
