package role

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommand(t *testing.T) {
	tempDir := t.TempDir()
	templateFile := filepath.Join(tempDir, "templates", "aggregated-role.yaml")

	// Sample template content for testing
	templateContent := `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kyverno-{{.Name}}-permission
  labels:
      {{- range .Controllers }}
      rbac.kyverno.io/aggregate-to-{{ . }}: "true"
      {{- end }}
rules:
  - apiGroups: ["{{.ApiGroup}}"]
    resources: ["{{.ResourceTypes | join \",\"}}"]
    verbs: [{{range .Verbs}}"{{.}}",{{end}}]
`

	// Write the template file to the temporary directory
	err := os.MkdirAll(filepath.Dir(templateFile), os.ModePerm)
	assert.NoError(t, err)
	err = os.WriteFile(templateFile, []byte(templateContent), 0644)
	assert.NoError(t, err)

	// Define test cases
	tests := []struct {
		name         string
		args         []string
		expectedFile string
		errorMsg     string
	}{
		{
			name:         "ValidCommandWithMultipleControllers",
			args:         []string{"name1", "--resources=crontabs", "--api-groups=stable.example.com", "--verbs=get,list", "--controllers=controller1", "--controllers=controller2"},
			expectedFile: "stdout",
		},
		{
			name:         "ValidCommandWithDefaultController",
			args:         []string{"name2", "--resources=pods", "--api-groups=core", "--verbs=get,list"},
			expectedFile: "stdout",
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
		},
		{
			name:         "SingleVerb",
			args:         []string{"name12", "--resources=pods", "--api-groups=core", "--verbs=get"},
			expectedFile: "stdout",
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
		},
		{
			name:         "ValidCommandWithMixedControllers",
			args:         []string{"name15", "--resources=pods", "--api-groups=core", "--verbs=get,list", "--controllers=controller1,controller2"},
			expectedFile: "stdout",
		},

		{
			name:         "AllFlagsWithComplexInput",
			args:         []string{"nameComplex", "--resources=pods,services", "--api-groups=core", "--verbs=get,list"},
			expectedFile: "stdout",
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
			err = cmd.Execute()
			if tc.errorMsg != "" {
				assert.ErrorContains(t, err, tc.errorMsg)
				return
			}
			assert.NoError(t, err)

			// Check the output based on expected result
			if tc.expectedFile == "stdout" {
				output := stdoutBuffer.String()
				assert.Contains(t, output, fmt.Sprintf("name: kyverno-%s-permission", tc.args[0]))
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
