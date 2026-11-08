package values

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
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
	expected := `Error: unknown command "foo" for "values"`
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

func TestCommandComplexValuesMerging(t *testing.T) {
	cmd := Command()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"-g", "url=https://example.com?query=val&mode=dev",
		"-g", "simple=val",
		"--rule", "mypol,rule1,env=prod",
		"--rule", "mypol,rule2,env=dev,debug=true",
		"--resource", "mypol,res1,app=nginx",
		"--rule", "otherpol,check,k=v=w",
		"-n", "prod,env=prod",
		"-n", "prod,region=us-east",
		"-n", "staging,tier=backend",
	})
	err := cmd.Execute()
	assert.NoError(t, err)

	var spec v1alpha1.ValuesSpec
	err = yaml.Unmarshal(buf.Bytes(), &spec)
	assert.NoError(t, err)

	// Global values: '=' must be preserved
	assert.Equal(t, "https://example.com?query=val&mode=dev", spec.GlobalValues["url"])
	assert.Equal(t, "val", spec.GlobalValues["simple"])

	// Policies must be deduplicated and merged by name
	assert.Len(t, spec.Policies, 2)

	// Verify mypol
	var mypol *v1alpha1.Policy
	for i := range spec.Policies {
		if spec.Policies[i].Name == "mypol" {
			mypol = &spec.Policies[i]
			break
		}
	}
	assert.NotNil(t, mypol)
	assert.Len(t, mypol.Rules, 2)
	assert.Equal(t, "rule1", mypol.Rules[0].Name)
	assert.Equal(t, "prod", mypol.Rules[0].Values["env"])
	assert.Equal(t, "rule2", mypol.Rules[1].Name)
	assert.Equal(t, "dev", mypol.Rules[1].Values["env"])
	assert.Equal(t, true, mypol.Rules[1].Values["debug"])
	assert.Len(t, mypol.Resources, 1)
	assert.Equal(t, "res1", mypol.Resources[0].Name)
	assert.Equal(t, "nginx", mypol.Resources[0].Values["app"])

	// Verify otherpol preserving '=' in rule value
	var otherpol *v1alpha1.Policy
	for i := range spec.Policies {
		if spec.Policies[i].Name == "otherpol" {
			otherpol = &spec.Policies[i]
			break
		}
	}
	assert.NotNil(t, otherpol)
	assert.Len(t, otherpol.Rules, 1)
	assert.Equal(t, "v=w", otherpol.Rules[0].Values["k"])

	// Namespace selectors must be deduplicated and merged by name
	assert.Len(t, spec.NamespaceSelectors, 2)
	var prodNs *v1alpha1.NamespaceSelector
	for i := range spec.NamespaceSelectors {
		if spec.NamespaceSelectors[i].Name == "prod" {
			prodNs = &spec.NamespaceSelectors[i]
			break
		}
	}
	assert.NotNil(t, prodNs)
	assert.Equal(t, "prod", prodNs.Labels["env"])
	assert.Equal(t, "us-east", prodNs.Labels["region"])
}

func TestCommandSameNameRuleAndResourceMerging(t *testing.T) {
	cmd := Command()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"--rule", "testpol,rule1,k1=v1,k2=initial",
		"--rule", "testpol,rule1,k2=overwritten,k3=v3",
		"--resource", "testpol,res1,r1=v1,r2=initial",
		"--resource", "testpol,res1,r2=overwritten,r3=v3",
	})
	err := cmd.Execute()
	assert.NoError(t, err)

	var spec v1alpha1.ValuesSpec
	err = yaml.Unmarshal(buf.Bytes(), &spec)
	assert.NoError(t, err)

	assert.Len(t, spec.Policies, 1)
	pol := spec.Policies[0]
	assert.Equal(t, "testpol", pol.Name)

	// Single merged rule
	assert.Len(t, pol.Rules, 1)
	assert.Equal(t, "rule1", pol.Rules[0].Name)
	assert.Equal(t, "v1", pol.Rules[0].Values["k1"])
	assert.Equal(t, "overwritten", pol.Rules[0].Values["k2"])
	assert.Equal(t, "v3", pol.Rules[0].Values["k3"])

	// Single merged resource
	assert.Len(t, pol.Resources, 1)
	assert.Equal(t, "res1", pol.Resources[0].Name)
	assert.Equal(t, "v1", pol.Resources[0].Values["r1"])
	assert.Equal(t, "overwritten", pol.Resources[0].Values["r2"])
	assert.Equal(t, "v3", pol.Resources[0].Values["r3"])
}

func TestCommandOutputToFile(t *testing.T) {
	tempDir := t.TempDir()
	outFile := filepath.Join(tempDir, "values.yaml")

	cmd := Command()
	cmd.SetArgs([]string{
		"-o", outFile,
		"-g", "env=test",
		"--rule", "testpol,rule1,a=b",
	})
	err := cmd.Execute()
	assert.NoError(t, err)

	content, err := os.ReadFile(outFile)
	assert.NoError(t, err)

	var spec v1alpha1.ValuesSpec
	err = yaml.Unmarshal(content, &spec)
	assert.NoError(t, err)
	assert.Equal(t, "test", spec.GlobalValues["env"])
	assert.Len(t, spec.Policies, 1)
	assert.Equal(t, "testpol", spec.Policies[0].Name)
}

func TestCommandErrorDoesNotTruncateFile(t *testing.T) {
	tempDir := t.TempDir()
	existingFile := filepath.Join(tempDir, "existing.yaml")
	originalContent := []byte("original: content\ndo: not truncate\n")
	err := os.WriteFile(existingFile, originalContent, 0644)
	assert.NoError(t, err)

	cmd := Command()
	cmd.SetArgs([]string{
		"-o", existingFile,
		"--rule", "invalid_rule_format",
	})
	err = cmd.Execute()
	assert.Error(t, err)

	// Existing file must be untouched
	content, err := os.ReadFile(existingFile)
	assert.NoError(t, err)
	assert.Equal(t, originalContent, content)
}

func TestCommandValidationErrors(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		errorMsg string
	}{
		{
			name:     "InvalidGlobalNoEquals",
			args:     []string{"-g", "invalid"},
			errorMsg: "invalid key=value pair: invalid",
		},
		{
			name:     "InvalidGlobalEmptyKey",
			args:     []string{"-g", "=value"},
			errorMsg: "invalid key=value pair: =value",
		},
		{
			name:     "InvalidRuleTooFewParts",
			args:     []string{"--rule", "mypol"},
			errorMsg: "invalid rule format: mypol",
		},
		{
			name:     "InvalidRuleNoKeyValue",
			args:     []string{"--rule", "mypol,rule1,nokeyvalue"},
			errorMsg: "invalid key=value pair: nokeyvalue",
		},
		{
			name:     "InvalidResourceTooFewParts",
			args:     []string{"--resource", "mypol"},
			errorMsg: "invalid resource format: mypol",
		},
		{
			name:     "InvalidNamespaceSelectorTooFewParts",
			args:     []string{"-n", "prod"},
			errorMsg: "invalid namespace selector format: prod",
		},
		{
			name:     "InvalidNamespaceSelectorNoKeyValue",
			args:     []string{"-n", "prod,nokeyvalue"},
			errorMsg: "invalid key=value pair: nokeyvalue",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := Command()
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.errorMsg)
		})
	}
}
