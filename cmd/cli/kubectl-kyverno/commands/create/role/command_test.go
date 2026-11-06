package role

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/yaml"
)

func TestCommand(t *testing.T) {
	tempDir := t.TempDir()

	// Define test cases
	tests := []struct {
		name             string
		args             []string
		expectedFile     string
		expectedContains []string
		errorMsg         string
	}{
		{
			name:         "ValidCommandWithMultipleControllers",
			args:         []string{"name1", "--resources=crontabs", "--api-groups=stable.example.com", "--verbs=get,list", "--controllers=controller1", "--controllers=controller2"},
			expectedFile: "stdout",
			expectedContains: []string{
				`rbac.kyverno.io/aggregate-to-controller1: "true"`,
				`rbac.kyverno.io/aggregate-to-controller2: "true"`,
				`resources: ["crontabs"]`,
				`verbs: ["get", "list"]`,
			},
		},
		{
			name:         "ValidCommandWithDefaultController",
			args:         []string{"name2", "--resources=pods", "--api-groups=core", "--verbs=get,list"},
			expectedFile: "stdout",
			expectedContains: []string{
				`rbac.kyverno.io/aggregate-to-background-controller: "true"`,
				`resources: ["pods"]`,
				`verbs: ["get", "list"]`,
			},
		},
		{
			name:     "MissingResources",
			args:     []string{"name3", "--api-groups=stable.example.com", "--verbs=get,list"},
			errorMsg: "required flag(s) \"resources\" not set",
		},
		{
			name:     "MissingApiGroup",
			args:     []string{"name4", "--resources=crontabs", "--verbs=get,list"},
			errorMsg: "required flag(s) \"api-groups\" not set",
		},
		{
			name:     "MissingVerbs",
			args:     []string{"name5", "--resources=crontabs", "--api-groups=stable.example.com"},
			errorMsg: "required flag(s) \"verbs\" not set",
		},
		{
			name:         "AllVerbExpands",
			args:         []string{"name6", "--resources=pods", "--api-groups=core", "--verbs=all"},
			expectedFile: "stdout",
			expectedContains: []string{
				`resources: ["pods"]`,
				`verbs: ["create", "get", "update", "delete", "list", "watch"]`,
			},
		},
		{
			name:         "OutputToFile",
			args:         []string{"name7", "--resources=pods", "--api-groups=core", "--verbs=get,list", "--output=" + filepath.Join(tempDir, "test-output.yaml")},
			expectedFile: "test-output.yaml",
		},
		{
			name:     "NoFlags",
			args:     []string{"name10"},
			errorMsg: "required flag(s) \"api-groups\", \"resources\", \"verbs\" not set",
		},
		{
			name:     "InvalidController",
			args:     []string{"name8", "--resources=pods", "--api-groups=core", "--verbs=get,list", "--controllers="},
			errorMsg: "invalid controller provided",
		},
		{
			name:         "MultipleResources",
			args:         []string{"name11", "--resources=pods,services", "--api-groups=core", "--verbs=get,list"},
			expectedFile: "stdout",
			expectedContains: []string{
				`resources: ["pods", "services"]`,
				`verbs: ["get", "list"]`,
			},
		},
		{
			name:         "SingleVerb",
			args:         []string{"name12", "--resources=pods", "--api-groups=core", "--verbs=get"},
			expectedFile: "stdout",
			expectedContains: []string{
				`verbs: ["get"]`,
			},
		},
		{
			name:     "NoApiGroup",
			args:     []string{"name13", "--resources=pods", "--verbs=get"},
			errorMsg: "required flag(s) \"api-groups\" not set",
		},
		{
			name:     "EmptyName",
			args:     []string{"", "--resources=pods", "--api-groups=stable.example.com", "--verbs=get,list"},
			errorMsg: "name argument is required",
		},
		{
			name:         "DifferentVerbCombinations",
			args:         []string{"name14", "--resources=pods", "--api-groups=core", "--verbs=create,delete"},
			expectedFile: "stdout",
			expectedContains: []string{
				`verbs: ["create", "delete"]`,
			},
		},
		{
			name:         "ValidCommandWithMixedControllers",
			args:         []string{"name15", "--resources=pods", "--api-groups=core", "--verbs=get,list", "--controllers=controller1,controller2"},
			expectedFile: "stdout",
			expectedContains: []string{
				`rbac.kyverno.io/aggregate-to-controller1: "true"`,
				`rbac.kyverno.io/aggregate-to-controller2: "true"`,
			},
		},
		{
			name:         "AllFlagsWithComplexInput",
			args:         []string{"nameComplex", "--resources=pods,services", "--api-groups=core", "--verbs=get,list"},
			expectedFile: "stdout",
			expectedContains: []string{
				`resources: ["pods", "services"]`,
				`verbs: ["get", "list"]`,
			},
		},
		{
			name:     "OutputFileCreationFailure",
			args:     []string{"nameOutputFail", "--resources=pods", "--api-groups=core", "--verbs=get,list", "--output=/invalid/path/test-output.yaml"},
			errorMsg: "failed to create file: ",
		},
		{
			name:         "SpecialCharacterName",
			args:         []string{"name@#%", "--resources=pods", "--api-groups=core", "--verbs=get"},
			expectedFile: "stdout",
		},
	}

	// Iterate over the test cases
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := Command()
			cmd.SetArgs(tc.args)

			// Prepare a buffer to capture stdout
			var stdoutBuffer bytes.Buffer
			cmd.SetOut(&stdoutBuffer)

			// Execute the command and handle errors
			err := cmd.Execute()
			if tc.errorMsg != "" {
				assert.ErrorContains(t, err, tc.errorMsg)
				return
			}
			assert.NoError(t, err)

			// Check the output based on expected result
			if tc.expectedFile == "stdout" {
				output := stdoutBuffer.String()
				assert.Contains(t, output, fmt.Sprintf("name: kyverno-%s-permission", tc.args[0]))
				for _, expected := range tc.expectedContains {
					assert.Contains(t, output, expected)
				}
			} else {
				expectedFilePath := filepath.Join(tempDir, tc.expectedFile)
				_, err := os.Stat(expectedFilePath)
				assert.NoError(t, err)

				// Clean up the created file
				_ = os.Remove(expectedFilePath)
			}
		})
	}
}

func TestClusterRoleValidYAML(t *testing.T) {
	cmd := Command()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{
		"test-role",
		"--api-groups=apps",
		"--resources=deployments,statefulsets",
		"--verbs=get,list,watch",
		"--controllers=background-controller,reports-controller",
	})
	err := cmd.Execute()
	assert.NoError(t, err)

	var role rbacv1.ClusterRole
	err = yaml.Unmarshal(buf.Bytes(), &role)
	assert.NoError(t, err)
	assert.Equal(t, "kyverno-test-role-permission", role.Name)
	assert.Equal(t, "ClusterRole", role.Kind)
	assert.Equal(t, "rbac.authorization.k8s.io/v1", role.APIVersion)
	assert.Equal(t, "true", role.Labels["rbac.kyverno.io/aggregate-to-background-controller"])
	assert.Equal(t, "true", role.Labels["rbac.kyverno.io/aggregate-to-reports-controller"])
	assert.Len(t, role.Rules, 1)
	assert.Equal(t, []string{"apps"}, role.Rules[0].APIGroups)
	assert.Equal(t, []string{"deployments", "statefulsets"}, role.Rules[0].Resources)
	assert.Equal(t, []string{"get", "list", "watch"}, role.Rules[0].Verbs)
}
