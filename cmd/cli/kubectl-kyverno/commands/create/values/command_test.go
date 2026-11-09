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

// TestCommand verifies default command execution without arguments.
func TestCommand(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	err := cmd.Execute()
	assert.NoError(t, err)
}

// TestCommandWithArgs verifies that positional arguments return an error.
func TestCommandWithArgs(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	cmd.SetArgs([]string{"foo"})
	err := cmd.Execute()
	assert.Error(t, err)
}

// TestCommandWithInvalidArg verifies error messages when invalid arguments are provided.
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

// TestCommandWithInvalidFlag verifies error messages when unknown flags are provided.
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

// TestCommandHelp verifies that the help flag outputs command usage.
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

// TestCommandComplexValuesMerging verifies hierarchical merging of policies, rules, resources, and selectors, and '=' preservation.
func TestCommandComplexValuesMerging(t *testing.T) {
	cmd := Command()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"-g", "url=https://example.com?query=val&mode=dev",
		"-g", "simple=val",
		"-g", "empty=",
		"--rule", "mypol,rule1,env=prod,emptyrule=",
		"--rule", "mypol,rule2,env=dev,debug=true",
		"--resource", "mypol,res1,app=nginx,emptyres=",
		"--rule", "otherpol,check,k=v=w",
		"-n", "prod,env=prod",
		"-n", "prod,region=us-east,emptyns=",
		"-n", "staging,tier=backend",
	})
	err := cmd.Execute()
	assert.NoError(t, err)

	var spec v1alpha1.ValuesSpec
	err = yaml.Unmarshal(buf.Bytes(), &spec)
	assert.NoError(t, err)

	// Global values: '=' must be preserved and empty values rendered as empty strings
	assert.Equal(t, "https://example.com?query=val&mode=dev", spec.GlobalValues["url"])
	assert.Equal(t, "val", spec.GlobalValues["simple"])
	assert.Equal(t, "", spec.GlobalValues["empty"])

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
	assert.Equal(t, "", mypol.Rules[0].Values["emptyrule"])
	assert.Equal(t, "rule2", mypol.Rules[1].Name)
	assert.Equal(t, "dev", mypol.Rules[1].Values["env"])
	assert.Equal(t, true, mypol.Rules[1].Values["debug"])
	assert.Len(t, mypol.Resources, 1)
	assert.Equal(t, "res1", mypol.Resources[0].Name)
	assert.Equal(t, "nginx", mypol.Resources[0].Values["app"])
	assert.Equal(t, "", mypol.Resources[0].Values["emptyres"])

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
	assert.Equal(t, "", prodNs.Labels["emptyns"])
}

// TestCommandSameNameRuleAndResourceMerging verifies that repeated rules and resources under the same policy merge correctly.
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

// TestCommandYAMLSensitiveValues verifies that YAML-sensitive characters are quoted while native scalar typing is preserved.
func TestCommandYAMLSensitiveValues(t *testing.T) {
	cmd := Command()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"--rule", "testpol,rule1,query=a=b #suffix",
		"--rule", "testpol,rule1,tabbed=a=b\t#suffix",
		"--rule", "testpol,rule1,mapping=key: value",
		"--rule", "testpol,rule1,comment=#standalone",
		"--rule", "testpol,rule1,alias=*missing",
		"--rule", "testpol,rule1,anchor=&base",
		"--rule", "testpol,rule1,padded=  spaced  ",
		"--rule", "testpol,rule1,multiline=line1\nline2",
		"--rule", "testpol,rule1,bracket=[abc=def",
		"--rule", "testpol,rule1,brace={key=val",
		"--rule", "testpol,rule1,pipe=|block",
		"--rule", "testpol,rule1,fold=>block",
		"--rule", "testpol,rule1,directive=%YAML",
		"--rule", "testpol,rule1,at=@mention",
		"--rule", "testpol,rule1,tag=!ref",
		"--rule", "testpol,rule1,question=?query",
		"--rule", "testpol,rule1,dash=- item",
		"--rule", "testpol,rule1,bool_true=true",
		"--rule", "testpol,rule1,bool_false=false",
		"--rule", "testpol,rule1,num_int=42",
		"--rule", "testpol,rule1,num_float=3.14",
		"--rule", "testpol,rule1,null_val=null",
	})
	err := cmd.Execute()
	assert.NoError(t, err)

	var spec v1alpha1.ValuesSpec
	err = yaml.Unmarshal(buf.Bytes(), &spec)
	assert.NoError(t, err)

	assert.Len(t, spec.Policies, 1)
	assert.Len(t, spec.Policies[0].Rules, 1)
	values := spec.Policies[0].Rules[0].Values
	assert.Equal(t, "a=b #suffix", values["query"])
	assert.Equal(t, "a=b\t#suffix", values["tabbed"])
	assert.Equal(t, "key: value", values["mapping"])
	assert.Equal(t, "#standalone", values["comment"])
	assert.Equal(t, "*missing", values["alias"])
	assert.Equal(t, "&base", values["anchor"])
	assert.Equal(t, "  spaced  ", values["padded"])
	assert.Equal(t, "line1\nline2", values["multiline"])
	assert.Equal(t, "[abc=def", values["bracket"])
	assert.Equal(t, "{key=val", values["brace"])
	assert.Equal(t, "|block", values["pipe"])
	assert.Equal(t, ">block", values["fold"])
	assert.Equal(t, "%YAML", values["directive"])
	assert.Equal(t, "@mention", values["at"])
	assert.Equal(t, "!ref", values["tag"])
	assert.Equal(t, "?query", values["question"])
	assert.Equal(t, "- item", values["dash"])
	assert.Equal(t, true, values["bool_true"])
	assert.Equal(t, false, values["bool_false"])
	assert.Equal(t, float64(42), values["num_int"])
	assert.Equal(t, 3.14, values["num_float"])
	assert.Nil(t, values["null_val"])
}

// TestCommandOutputToFile verifies writing generated values to an output file.
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

// TestCommandErrorDoesNotTruncateFile verifies that validation failures do not overwrite or truncate an existing file.
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

// TestCommandValidationErrors verifies descriptive error reporting for malformed command inputs.
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
			name:     "InvalidGlobalUTF8",
			args:     []string{"-g", "bad=\xff\xfe"},
			errorMsg: "invalid UTF-8 in key=value pair",
		},
		{
			name:     "InvalidRuleUTF8",
			args:     []string{"--rule", "mypol,rule1,bad=\xff\xfe"},
			errorMsg: "invalid UTF-8 in key=value pair",
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
			name:     "InvalidRuleEmptyKey",
			args:     []string{"--rule", "mypol,rule1,=value"},
			errorMsg: "invalid key=value pair: =value",
		},
		{
			name:     "InvalidResourceTooFewParts",
			args:     []string{"--resource", "mypol"},
			errorMsg: "invalid resource format: mypol",
		},
		{
			name:     "InvalidResourceNoKeyValue",
			args:     []string{"--resource", "mypol,res1,nokeyvalue"},
			errorMsg: "invalid key=value pair: nokeyvalue",
		},
		{
			name:     "InvalidResourceEmptyKey",
			args:     []string{"--resource", "mypol,res1,=value"},
			errorMsg: "invalid key=value pair: =value",
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
		{
			name:     "InvalidNamespaceSelectorEmptyKey",
			args:     []string{"-n", "prod,=value"},
			errorMsg: "invalid key=value pair: =value",
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

// TestCommandNamespaceLabelTypes verifies that namespace selector labels are always quoted as strings,
// preventing YAML unmarshaling errors into map[string]string for boolean or numeric labels.
func TestCommandNamespaceLabelTypes(t *testing.T) {
	cmd := Command()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"-n", "prod,env=true,tier=1,active=false,name=production",
	})
	err := cmd.Execute()
	assert.NoError(t, err)

	yamlStr := buf.String()
	assert.Contains(t, yamlStr, `env: "true"`)
	assert.Contains(t, yamlStr, `tier: "1"`)
	assert.Contains(t, yamlStr, `active: "false"`)
	assert.Contains(t, yamlStr, `name: "production"`)

	var spec v1alpha1.ValuesSpec
	err = yaml.UnmarshalStrict(buf.Bytes(), &spec)
	assert.NoError(t, err)

	assert.Len(t, spec.NamespaceSelectors, 1)
	labels := spec.NamespaceSelectors[0].Labels
	assert.Equal(t, "true", labels["env"])
	assert.Equal(t, "1", labels["tier"])
	assert.Equal(t, "false", labels["active"])
	assert.Equal(t, "production", labels["name"])
}

// TestCommandGlobalValueWithCommas verifies that --global uses StringArrayVarP so values with commas (e.g., URLs) are not split.
func TestCommandGlobalValueWithCommas(t *testing.T) {
	cmd := Command()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"-g", "url=https://example.com?a=1,b=2",
		"-g", "list=item1,item2,item3",
	})
	err := cmd.Execute()
	assert.NoError(t, err)

	var spec v1alpha1.ValuesSpec
	err = yaml.UnmarshalStrict(buf.Bytes(), &spec)
	assert.NoError(t, err)

	assert.Equal(t, "https://example.com?a=1,b=2", spec.GlobalValues["url"])
	assert.Equal(t, "item1,item2,item3", spec.GlobalValues["list"])
}

// TestCommandEmptyRuleAndResourceValues verifies that rules and resources without key-value pairs emit values: {} instead of null.
func TestCommandEmptyRuleAndResourceValues(t *testing.T) {
	cmd := Command()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"--rule", "mypol,rule1",
		"--resource", "mypol,res1",
	})
	err := cmd.Execute()
	assert.NoError(t, err)

	yamlStr := buf.String()
	assert.Contains(t, yamlStr, "values: {}")

	var spec v1alpha1.ValuesSpec
	err = yaml.UnmarshalStrict(buf.Bytes(), &spec)
	assert.NoError(t, err)

	assert.Len(t, spec.Policies, 1)
	pol := spec.Policies[0]
	assert.Len(t, pol.Rules, 1)
	assert.Equal(t, "rule1", pol.Rules[0].Name)
	assert.NotNil(t, pol.Rules[0].Values)
	assert.Empty(t, pol.Rules[0].Values)

	assert.Len(t, pol.Resources, 1)
	assert.Equal(t, "res1", pol.Resources[0].Name)
	assert.NotNil(t, pol.Resources[0].Values)
	assert.Empty(t, pol.Resources[0].Values)
}
