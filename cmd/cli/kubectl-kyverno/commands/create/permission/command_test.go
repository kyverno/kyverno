package permission

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommand(t *testing.T) {
	tempDir := t.TempDir()
	templateFile := filepath.Join(tempDir, "templates", "aggregated-role.yaml")

	// Sample template content
	templateContent := `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kyverno-{{.ResourceType}}-permission
  labels:
      {{- range .Controllers }}
      rbac.kyverno.io/aggregate-to-{{ . }}: "true"
      {{- end }}
rules:
  - apiGroups: ["{{.ApiGroup}}"]
    resources: ["{{.ResourceType}}"]
    verbs: [{{- range .Verbs }}"{{ . }}{{- if not (eq . $.Verbs[len $.Verbs - 1]) }},{{ end }}{{- end }}]
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
		controllers  []string
		verbs        []string
		apiGroup     string
		errorMsg     string
	}{
		{
			name:         "ValidCommandWithMultipleControllers",
			args:         []string{"crontabs", "--api-group=stable.example.com", "--verbs=get,list", "--controllers=controller1", "--controllers=controller2"},
			expectedFile: "stdout",
			controllers:  []string{"controller1", "controller2"},
			verbs:        []string{"get", "list"},
			apiGroup:     "stable.example.com",
		},
		{
			name:     "MissingResourceType",
			args:     []string{"", "--api-group=stable.example.com", "--verbs=get,list"},
			errorMsg: "the resource type argument is required",
		},
		{
			name:     "MissingApiGroup",
			args:     []string{"crontabs", "--verbs=get,list"},
			errorMsg: "required flag(s) \"api-group\" not set",
		},
		{
			name:     "MissingVerbs",
			args:     []string{"crontabs", "--api-group=stable.example.com"},
			errorMsg: "required flag(s) \"verbs\" not set",
		},
		{
			name:         "AllVerbExpands",
			args:         []string{"pods", "--api-group=core", "--verbs=all"},
			expectedFile: "stdout",
		},
		{
			name:         "OutputToFile",
			args:         []string{"pods", "--api-group=core", "--verbs=get,list", "--output=" + filepath.Join(tempDir, "test-output.yaml")},
			expectedFile: "test-output.yaml",
		},
	}

	// Iterate over the test cases
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := Command()
			cmd.SetArgs(tc.args)

			// Execute the command and handle errors
			err = cmd.Execute()
			if tc.errorMsg != "" {
				assert.ErrorContains(t, err, tc.errorMsg)
				return
			}

			assert.NoError(t, err)

			// Check the output based on expected result
			if tc.expectedFile == "stdout" {
				// Capture stdout
				output, err := captureStdout(func() {
					cmd.Execute()
				})
				assert.NoError(t, err)
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

// Helper function to capture stdout
func captureStdout(f func()) (string, error) {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf [4096]byte
	n, err := r.Read(buf[:])
	if err != nil {
		return "", err
	}

	return string(buf[:n]), nil
}
