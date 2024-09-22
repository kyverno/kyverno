package permission

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommand(t *testing.T) {
	tempDir := t.TempDir()
	templateFile := filepath.Join(tempDir, "templates", "aggregated-role.yaml")

	// Sample template content with corrected subjects loop
	templateContent := `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kyverno-{{.ResourceType}}-permission
  labels:
    rbac.authorization.k8s.io/aggregate-to-kyverno: "true"
rules:
  - apiGroups: ["{{.ApiGroup}}"]
    resources: ["{{.ResourceType}}"]
    verbs: [{{range .Verbs}}"{{.}}",{{end}}]

---

apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{.ResourceType}}-binding
subjects:
{{- range .Controllers }}
- kind: ServiceAccount
  name: {{ . }}
  namespace: kyverno
{{- end }}
roleRef:
  kind: ClusterRole
  name: kyverno-{{ .ResourceType }}-permission
  apiGroup: rbac.authorization.k8s.io
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
		resourceType string
		errorMsg     string
	}{
		{
			name:         "ValidCommandWithMultipleControllers",
			args:         []string{"crontabs", "--api-group=stable.example.com", "--verbs=get,list", "--controllers=controller1", "--controllers=controller2"},
			expectedFile: "crontabs-permission.yaml",
			controllers:  []string{"controller1", "controller2"},
			verbs:        []string{"get", "list"},
			apiGroup:     "stable.example.com",
			resourceType: "crontabs",
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
			expectedFile: "pods-permission.yaml",
		},
	}

	// Iterate over the test cases
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := Command() // Create the *cobra.Command
			cmd.SetArgs(tc.args)

			// Change to temporary directory
			err = os.Chdir(tempDir)
			assert.NoError(t, err)

			// Execute the command and handle errors
			err = cmd.Execute()
			if tc.errorMsg != "" {
				assert.ErrorContains(t, err, tc.errorMsg)
				return
			}

			assert.NoError(t, err)

			// Verify the file was created as expected
			homeDir, err := os.UserHomeDir()
			assert.NoError(t, err)
			expectedFilePath := filepath.Join(homeDir, "aggregated-role", tc.expectedFile)
			_, err = os.Stat(expectedFilePath)
			assert.NoError(t, err)

			// Verify the controllers in the file (if provided)
			if len(tc.controllers) > 0 {
				content, err := os.ReadFile(expectedFilePath)
				assert.NoError(t, err)
				for _, sa := range tc.controllers {
					assert.Contains(t, string(content), fmt.Sprintf("name: %s", sa))
				}
			}

			// Clean up
			_ = os.Remove(expectedFilePath)
		})
	}
}

// Mock exec.Command for testing kubectl apply without an actual Kubernetes cluster
var execCommand = func(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestApplyManifest(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test-manifest.yaml")

	// Create a dummy manifest file for the test
	manifestContent := `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: nginx
    image: nginx
`
	// Write the manifest content to the temporary file
	err := os.WriteFile(filePath, []byte(manifestContent), 0644)
	assert.NoError(t, err, "Failed to create test-manifest.yaml file")

	// Mock the `kubectl apply` command
	execCommand = func(command string, args ...string) *exec.Cmd {
		cs := append([]string{"-test.run=TestHelperProcess", "--", command}, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	// Apply the manifest using the ApplyManifest function
	err = ApplyManifest(filePath)
	assert.NoError(t, err, "Failed to apply the manifest")

	// Verify that the manifest file exists before removing it
	_, err = os.Stat(filePath)
	assert.NoError(t, err, "Manifest file should exist before cleanup")

	// Clean up the dummy file
	err = os.Remove(filePath)
	assert.NoError(t, err, "Failed to clean up test-manifest.yaml")
}

func TestHelperProcess(*testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	fmt.Println("kubectl apply -f test-manifest.yaml")
	os.Exit(0)
}
func TestCommandIntegration(t *testing.T) {
	// Set up a temporary Kubernetes context (consider using Kind/Minikube)
	// Make sure you have a running cluster available

	// Sample test parameters
	resourceType := "crontabs"
	apiGroup := "stable.example.com"
	verbs := "get,list"
	controller := "my-deployer"

	// Create the command
	cmd := Command()
	cmd.SetArgs([]string{resourceType, "--api-group=" + apiGroup, "--verbs=" + verbs, "--controllers=" + controller})

	// Execute the command
	err := cmd.Execute()
	assert.NoError(t, err)

	// Verify that the ClusterRole exists
	roleName := fmt.Sprintf("kyverno-%s-permission", resourceType)
	cmdRole := exec.Command("kubectl", "get", "clusterrole", roleName)
	if output, err := cmdRole.CombinedOutput(); err != nil {
		t.Fatalf("failed to get ClusterRole: %s, output: %s", err, output)
	}

	// Verify that the ClusterRoleBinding exists
	bindingName := fmt.Sprintf("%s-binding", resourceType)
	cmdBinding := exec.Command("kubectl", "get", "clusterrolebinding", bindingName)
	if output, err := cmdBinding.CombinedOutput(); err != nil {
		t.Fatalf("failed to get ClusterRoleBinding: %s, output: %s", err, output)
	}

	// Verify the binding's subjects

	// Clean up resources after the test
	defer exec.Command("kubectl", "delete", "clusterrole", roleName).Run()
	defer exec.Command("kubectl", "delete", "clusterrolebinding", bindingName).Run()
}
